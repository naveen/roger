package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/naveen/roger/relay/internal/audio"
	"github.com/naveen/roger/relay/internal/config"
	"github.com/naveen/roger/relay/internal/guard"
	"github.com/naveen/roger/relay/internal/pipeline"
	"github.com/naveen/roger/relay/internal/protocol"
	"github.com/naveen/roger/relay/internal/rules"
	"github.com/naveen/roger/relay/internal/store"
)

const testToken = "rgr_test_token"

// fakePipe is a scripted pipeline that records what it was asked.
type fakePipe struct {
	mu        sync.Mutex
	sttCalls  int
	systems   []string
	histories [][]pipeline.Message
	spoken    []string
	heard     string
	reply     pipeline.Reply
	audioDur  time.Duration
}

func (f *fakePipe) Transcribe(_ context.Context, pcm []byte, rate int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sttCalls++
	return f.heard, nil
}

func (f *fakePipe) Reply(_ context.Context, req pipeline.ReplyRequest) (pipeline.Reply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.systems = append(f.systems, req.System)
	f.histories = append(f.histories, req.History)
	return f.reply, nil
}

func (f *fakePipe) Speak(_ context.Context, text string) (int, io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spoken = append(f.spoken, text)
	return 24000, io.NopCloser(bytes.NewReader(audio.Tone(600, f.audioDur, 24000, 0.2))), nil
}

type rig struct {
	t    *testing.T
	srv  *Server
	pipe *fakePipe
	st   *store.Store
	dev  *config.Device
	url  string
	now  time.Time
}

func newRig(t *testing.T, r rules.Rules) *rig {
	t.Helper()
	dir := t.TempDir()
	sum := sha256.Sum256([]byte(testToken))
	devJSON, _ := json.Marshal(map[string]any{"devices": []map[string]any{{
		"id": "roger-01", "token_sha256": hex.EncodeToString(sum[:]),
		"kid": map[string]any{"name": "Sam", "age": 7}, "timezone": "America/Los_Angeles",
		"parent_email": "parent@example.com", "rules": r,
	}}})
	path := dir + "/devices.json"
	if err := writeFile(path, devJSON); err != nil {
		t.Fatal(err)
	}
	devs, err := config.LoadDevices(path)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := store.New(dir + "/data")
	pipe := &fakePipe{
		heard:    "why do worms come out when it rains",
		reply:    pipeline.Reply{Say: "Worms breathe through their skin. Over.", Topic: "worms and rain", Flag: "none"},
		audioDur: 400 * time.Millisecond,
	}
	srv := New(devs, st, pipeline.Pipeline{STT: pipe, LLM: pipe, TTS: pipe}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rg := &rig{t: t, srv: srv, pipe: pipe, st: st, dev: devs.Devices[0]}
	// 15:00 in Los Angeles: outside the default quiet hours.
	rg.now = time.Date(2026, 11, 16, 23, 0, 0, 0, time.UTC)
	srv.Now = func() time.Time { return rg.now }
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	rg.url = "ws" + strings.TrimPrefix(hs.URL, "http") + "/v1/radio"
	return rg
}

type radio struct {
	t *testing.T
	c *websocket.Conn
}

func (rg *rig) dial() *radio {
	rg.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, rg.url, &websocket.DialOptions{
		Subprotocols: []string{protocol.Subprotocol},
		HTTPHeader:   http.Header{"Authorization": {"Bearer " + testToken}, "Origin": {"file://"}},
	})
	if err != nil {
		rg.t.Fatal(err)
	}
	rg.t.Cleanup(func() { c.CloseNow() })
	r := &radio{t: rg.t, c: c}
	r.send(protocol.Envelope{Type: protocol.TypeHello, Proto: protocol.Subprotocol, FW: "test", Board: "test",
		Mic: &protocol.AudioSpec{Codec: protocol.CodecPCM16, Rate: 16000}})
	if m := r.next(); m.Type != protocol.TypeReady || m.Limits == nil {
		rg.t.Fatalf("want ready, got %+v", m)
	}
	return r
}

func (r *radio) send(m protocol.Envelope) {
	r.t.Helper()
	b, _ := json.Marshal(m)
	if err := r.c.Write(context.Background(), websocket.MessageText, b); err != nil {
		r.t.Fatal(err)
	}
}

func (r *radio) audio(d time.Duration) {
	r.t.Helper()
	if err := r.c.Write(context.Background(), websocket.MessageBinary, audio.Tone(440, d, 16000, 0.3)); err != nil {
		r.t.Fatal(err)
	}
}

// next returns the next text message, skipping (and counting) audio.
func (r *radio) next() protocol.Envelope {
	m, _ := r.nextCounting()
	return m
}

func (r *radio) nextCounting() (protocol.Envelope, int) {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n := 0
	for {
		typ, data, err := r.c.Read(ctx)
		if err != nil {
			r.t.Fatalf("read: %v", err)
		}
		if typ == websocket.MessageBinary {
			n += len(data)
			continue
		}
		var m protocol.Envelope
		json.Unmarshal(data, &m)
		return m, n
	}
}

func (r *radio) press(seq int, d time.Duration) {
	r.send(protocol.Envelope{Type: protocol.TypePTTDown, Seq: seq})
	r.audio(d)
	r.send(protocol.Envelope{Type: protocol.TypePTTUp, Seq: seq})
}

func TestUnauthorized(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	for _, hdr := range []string{"", "Bearer wrong", testToken} {
		h := http.Header{}
		if hdr != "" {
			h.Set("Authorization", hdr)
		}
		_, resp, err := websocket.Dial(context.Background(), rg.url, &websocket.DialOptions{Subprotocols: []string{protocol.Subprotocol}, HTTPHeader: h})
		if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Authorization %q: want 401, got resp=%v err=%v", hdr, resp, err)
		}
	}
}

func TestHappyPath(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	r := rg.dial()
	r.press(1, time.Second)

	if m := r.next(); m.Type != protocol.TypeThinking || m.Seq != 1 {
		t.Fatalf("want thinking, got %+v", m)
	}
	start := r.next()
	if start.Type != protocol.TypeReplyStart || start.Rate != 24000 || start.Codec != protocol.CodecPCM16 {
		t.Fatalf("want reply.start, got %+v", start)
	}
	end, n := r.nextCounting()
	if end.Type != protocol.TypeReplyEnd || end.Cancelled {
		t.Fatalf("want reply.end, got %+v", end)
	}
	if got := audio.Duration(n, 24000); got != 400*time.Millisecond {
		t.Errorf("received %v of audio, want 400ms", got)
	}

	turns, _ := rg.st.Day("roger-01", "2026-11-16")
	if len(turns) != 1 {
		t.Fatalf("stored %d turns, want 1", len(turns))
	}
	got := turns[0]
	if got.Heard != rg.pipe.heard || got.Topic != "worms and rain" || got.MicMS != 1000 || got.ReplyMS != 400 {
		t.Errorf("stored turn = %+v", got)
	}
	if !strings.Contains(rg.pipe.systems[0], "Sam, who is 7") {
		t.Error("system prompt was not built for this kid")
	}
}

func TestConversationHistoryCarriesOver(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	r := rg.dial()
	for seq := 1; seq <= 2; seq++ {
		r.press(seq, time.Second)
		for r.next().Type != protocol.TypeReplyEnd {
		}
	}
	if len(rg.pipe.histories[0]) != 0 || len(rg.pipe.histories[1]) != 2 {
		t.Errorf("history lengths = %d, %d; want 0, 2", len(rg.pipe.histories[0]), len(rg.pipe.histories[1]))
	}
	// After a long gap it is a new conversation.
	rg.now = rg.now.Add(historyIdle + time.Minute)
	r.press(3, time.Second)
	for r.next().Type != protocol.TypeReplyEnd {
	}
	if len(rg.pipe.histories[2]) != 0 {
		t.Errorf("history should reset after %v idle", historyIdle)
	}
}

func TestAudioOutsidePressClosesConnection(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	r := rg.dial()
	r.audio(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := r.c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("want policy violation close, got %v", err)
	}
	if rg.pipe.sttCalls != 0 {
		t.Error("audio sent without a press reached the transcriber")
	}
}

func TestQuietHours(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	rg.now = time.Date(2026, 11, 17, 5, 0, 0, 0, time.UTC) // 21:00 in Los Angeles
	r := rg.dial()

	r.press(1, time.Second)
	if m := r.next(); m.Type != protocol.TypeNotice || m.Code != protocol.NoticeQuietHours || !m.Speech {
		t.Fatalf("want spoken quiet_hours notice, got %+v", m)
	}
	if m := r.next(); m.Type != protocol.TypeReplyStart {
		t.Fatalf("want reply.start for the spoken notice, got %+v", m)
	}
	if m := r.next(); m.Type != protocol.TypeReplyEnd {
		t.Fatalf("want reply.end, got %+v", m)
	}

	// Mashing the button must not buy more speech.
	r.press(2, time.Second)
	if m := r.next(); m.Code != protocol.NoticeQuietHours || m.Speech {
		t.Fatalf("second notice should be silent, got %+v", m)
	}
	if rg.pipe.sttCalls != 0 {
		t.Error("a refused press was transcribed")
	}
	if len(rg.pipe.spoken) != 1 || rg.pipe.spoken[0] != lineQuietHours {
		t.Errorf("spoken = %q", rg.pipe.spoken)
	}
}

func TestAirtimeExhausted(t *testing.T) {
	r0 := rules.Defaults()
	r0.DailyAirtimeMinutes = 1
	rg := newRig(t, r0)
	rg.st.Append("roger-01", rg.dev.Location(), store.Turn{Time: rg.now, MicMS: 20000, ReplyMS: 45000})
	r := rg.dial()
	r.press(1, time.Second)
	if m := r.next(); m.Type != protocol.TypeNotice || m.Code != protocol.NoticeAirtimeExhausted {
		t.Fatalf("want airtime_exhausted, got %+v", m)
	}
}

func TestShortPressIsNotHeard(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	r := rg.dial()
	r.press(1, 100*time.Millisecond)
	if m := r.next(); m.Type != protocol.TypeNotice || m.Code != protocol.NoticeNotHeard || m.Speech {
		t.Fatalf("want silent not_heard, got %+v", m)
	}
	if rg.pipe.sttCalls != 0 {
		t.Error("a tap should not be transcribed")
	}
}

func TestSilenceIsNotHeard(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	rg.pipe.heard = ""
	r := rg.dial()
	r.press(1, time.Second)
	if m := r.next(); m.Type != protocol.TypeThinking {
		t.Fatalf("got %+v", m)
	}
	if m := r.next(); m.Code != protocol.NoticeNotHeard {
		t.Fatalf("want not_heard, got %+v", m)
	}
}

func TestNewPressCancelsReply(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	rg.pipe.audioDur = 5 * time.Second // long enough to still be streaming
	r := rg.dial()
	r.press(1, time.Second)
	for r.next().Type != protocol.TypeReplyStart {
	}

	r.send(protocol.Envelope{Type: protocol.TypePTTDown, Seq: 2})
	end, _ := r.nextCounting()
	if end.Type != protocol.TypeReplyEnd || end.Seq != 1 || !end.Cancelled {
		t.Fatalf("want cancelled reply.end for seq 1, got %+v", end)
	}
	r.audio(time.Second)
	r.send(protocol.Envelope{Type: protocol.TypePTTUp, Seq: 2})
	if m, n := r.nextCounting(); m.Type != protocol.TypeThinking || m.Seq != 2 || n != 0 {
		t.Fatalf("want thinking for seq 2 with no stray audio, got %+v after %d bytes", m, n)
	}
}

func TestGuardBlocksReply(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	rg.pipe.reply = pipeline.Reply{Say: "oh shit", Topic: "x", Flag: "none", Remember: "bad"}
	r := rg.dial()
	r.press(1, time.Second)
	for r.next().Type != protocol.TypeReplyEnd {
	}
	if rg.pipe.spoken[0] != guard.Fallback {
		t.Errorf("spoke %q, want the fallback line", rg.pipe.spoken[0])
	}
	turns, _ := rg.st.Day("roger-01", "2026-11-16")
	if turns[0].Flag != "needs_parent" {
		t.Errorf("blocked reply should be flagged for the parent, got %q", turns[0].Flag)
	}
	if rem, _ := rg.st.Remembered("roger-01"); len(rem) != 0 {
		t.Error("nothing should be remembered from a blocked reply")
	}
}

func TestRemember(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	rg.pipe.reply = pipeline.Reply{Say: "Got it. Over.", Topic: "fort plans", Flag: "none", Remember: "A fort with a slide exit"}
	r := rg.dial()
	r.press(1, time.Second)
	for r.next().Type != protocol.TypeReplyEnd {
	}
	r.press(2, time.Second)
	for r.next().Type != protocol.TypeReplyEnd {
	}
	if !strings.Contains(rg.pipe.systems[1], "A fort with a slide exit") {
		t.Error("remembered note was not offered back to the model")
	}
}

func TestNewConnectionReplacesOld(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	first := rg.dial()
	rg.dial()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := first.c.Read(ctx); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("old connection should be closed, got %v", err)
	}
}

func TestHousekeepSendsDigestOncePerDay(t *testing.T) {
	rg := newRig(t, rules.Defaults())
	rg.st.Append("roger-01", rg.dev.Location(), store.Turn{Time: rg.now, Heard: "can a cloud be heavy", Topic: "clouds", Flag: "none"})
	smtp := config.SMTP{Host: "smtp.example.com", From: "roger@example.com"}
	var sent []string
	send := func(_ config.SMTP, to, subject, html string) error {
		sent = append(sent, to+"|"+subject)
		if !strings.Contains(html, "clouds") {
			t.Error("digest body missing topic")
		}
		return nil
	}
	rg.srv.Housekeep(rg.now, smtp, send) // 15:00 local: before send_at
	if len(sent) != 0 {
		t.Fatal("digest sent before send_at")
	}
	evening := rg.now.Add(5 * time.Hour) // 20:00 local
	rg.srv.Housekeep(evening, smtp, send)
	rg.srv.Housekeep(evening.Add(time.Minute), smtp, send)
	if len(sent) != 1 || !strings.HasPrefix(sent[0], "parent@example.com|") {
		t.Fatalf("sent = %v, want exactly one", sent)
	}
	rg.srv.Housekeep(evening, config.SMTP{}, send) // unconfigured SMTP never sends
	if len(sent) != 1 {
		t.Error("sent without SMTP configured")
	}
}
