package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/naveen/roger/relay/internal/audio"
	"github.com/naveen/roger/relay/internal/config"
	"github.com/naveen/roger/relay/internal/guard"
	"github.com/naveen/roger/relay/internal/persona"
	"github.com/naveen/roger/relay/internal/pipeline"
	"github.com/naveen/roger/relay/internal/protocol"
	"github.com/naveen/roger/relay/internal/store"
)

type pressState int

const (
	idle      pressState = iota
	listening            // between an accepted ptt.down and its ptt.up
	rejected             // ptt.down was refused; drop audio until ptt.up
)

// Lines Roger speaks for notices. They stay in the fiction on purpose.
const (
	lineQuietHours = "Roger's off the air for the night. Sleep tight. Over and out."
	lineAirtime    = "That's all the air time for today. Go build something and tell me about it tomorrow. Over and out."
)

// session is one WebSocket connection from one radio.
//
// Logging rule: nothing a child says, and nothing Roger says back, is ever
// written to the log. Log lines carry ids, durations and flags only.
type session struct {
	srv *Server
	dev *config.Device
	c   *websocket.Conn
	log *slog.Logger
	ctx context.Context

	wmu     sync.Mutex
	micRate int

	// Press state. Touched only by the read loop.
	state    pressState
	seq      int
	buf      []byte
	maxBytes int

	// The turn in flight, if any.
	turnCancel context.CancelFunc
	turnDone   chan struct{}
}

func (s *session) run(ctx context.Context) {
	s.ctx = ctx
	defer s.cancelTurn()
	defer s.c.CloseNow()

	if err := s.handshake(); err != nil {
		s.log.Info("handshake failed", "err", err)
		s.c.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}
	s.srv.pruneOncePerDay(s.dev, s.srv.Now())
	s.log.Info("radio connected", "mic_rate", s.micRate)

	for {
		typ, data, err := s.c.Read(ctx)
		if err != nil {
			s.log.Info("radio disconnected", "reason", websocket.CloseStatus(err))
			return
		}
		if typ == websocket.MessageBinary {
			if !s.onAudio(data) {
				return
			}
			continue
		}
		var m protocol.Envelope
		if json.Unmarshal(data, &m) != nil {
			s.c.Close(websocket.StatusUnsupportedData, "text frames must be JSON")
			return
		}
		switch m.Type {
		case protocol.TypePTTDown:
			s.onDown(m.Seq)
		case protocol.TypePTTUp:
			s.onUp()
		case protocol.TypeCancel:
			s.cancelTurn()
		default:
			// Unknown types are ignored so newer firmware can talk to an
			// older relay.
		}
	}
}

func (s *session) handshake() error {
	ctx, cancel := context.WithTimeout(s.ctx, helloTimeout)
	defer cancel()
	typ, data, err := s.c.Read(ctx)
	if err != nil {
		return errors.New("no hello")
	}
	var m protocol.Envelope
	if typ != websocket.MessageText || json.Unmarshal(data, &m) != nil || m.Type != protocol.TypeHello {
		return errors.New("first message must be hello")
	}
	if m.Mic == nil || m.Mic.Codec != protocol.CodecPCM16 {
		return errors.New("mic.codec must be pcm16")
	}
	if m.Mic.Rate < 8000 || m.Mic.Rate > 48000 {
		return errors.New("mic.rate must be 8000-48000")
	}
	s.micRate = m.Mic.Rate
	s.maxBytes = int(MaxPress.Seconds()) * s.micRate * 2
	s.log = s.log.With("fw", m.FW, "board", m.Board)

	return s.send(protocol.Envelope{
		Type:    protocol.TypeReady,
		Session: newSessionID(),
		Limits: &protocol.Limits{
			MaxPressMS: int(MaxPress.Milliseconds()),
			VolumeCap:  s.dev.EffectiveRules().VolumeCap,
		},
	})
}

func (s *session) onDown(seq int) {
	s.cancelTurn()
	s.seq = seq
	s.buf = s.buf[:0]

	now := s.srv.Now()
	local := now.In(s.dev.Location())
	r := s.dev.EffectiveRules()

	code, line := "", ""
	if r.InQuietHours(local) {
		code, line = protocol.NoticeQuietHours, lineQuietHours
	} else {
		used, err := s.srv.Store.AirtimeUsed(s.dev.ID, local.Format("2006-01-02"))
		if err != nil {
			s.log.Error("airtime lookup failed", "err", err)
		}
		if r.AirtimeLeft(used) == 0 {
			code, line = protocol.NoticeAirtimeExhausted, lineAirtime
		}
	}
	if code == "" {
		s.state = listening
		return
	}

	s.state = rejected
	s.log.Info("press refused", "seq", seq, "code", code)
	s.record(store.Turn{Time: now, Seq: seq, Notice: code})
	speak := s.srv.allowSpokenNotice(s.dev.ID, now)
	s.send(protocol.Envelope{Type: protocol.TypeNotice, Seq: seq, Code: code, Speech: speak})
	if speak {
		s.startTurn(func(ctx context.Context) {
			if _, err := s.speak(ctx, seq, line); err != nil {
				s.log.Error("spoken notice failed", "seq", seq, "err", err)
			}
		})
	}
}

// onAudio returns false if the connection must be closed.
func (s *session) onAudio(data []byte) bool {
	switch s.state {
	case rejected:
		return true // refused press: drop until ptt.up
	case idle:
		// A radio that sends audio without a press is broken or lying about
		// its button. Either way the relay will not listen.
		s.log.Warn("audio outside a press; closing")
		s.c.Close(websocket.StatusPolicyViolation, "audio outside a press")
		return false
	}
	if len(data)%2 != 0 {
		s.c.Close(websocket.StatusUnsupportedData, "pcm16 frames must have an even length")
		return false
	}
	if room := s.maxBytes - len(s.buf); room < len(data) {
		data = data[:room] // past the press limit: keep what fits, drop the rest
	}
	s.buf = append(s.buf, data...)
	return true
}

func (s *session) onUp() {
	was := s.state
	s.state = idle
	if was != listening {
		return
	}
	seq := s.seq
	pcm := append([]byte(nil), s.buf...)
	clear(s.buf)
	s.buf = s.buf[:0]

	if audio.Duration(len(pcm), s.micRate) < MinPress {
		s.send(protocol.Envelope{Type: protocol.TypeNotice, Seq: seq, Code: protocol.NoticeNotHeard})
		return
	}
	s.startTurn(func(ctx context.Context) { s.turn(ctx, seq, pcm) })
}

func (s *session) startTurn(fn func(ctx context.Context)) {
	ctx, cancel := context.WithTimeout(s.ctx, turnTimeout)
	done := make(chan struct{})
	s.turnCancel, s.turnDone = cancel, done
	go func() {
		defer close(done)
		defer cancel()
		fn(ctx)
	}()
}

// cancelTurn stops the turn in flight and waits for it, so that nothing for
// an old seq is sent after a new press begins.
func (s *session) cancelTurn() {
	if s.turnCancel == nil {
		return
	}
	s.turnCancel()
	<-s.turnDone
	s.turnCancel, s.turnDone = nil, nil
}

// turn runs one press through the pipeline.
func (s *session) turn(ctx context.Context, seq int, pcm []byte) {
	start := s.srv.Now()
	began := time.Now()
	micMS := int(audio.Duration(len(pcm), s.micRate).Milliseconds())
	t := store.Turn{Time: start, Seq: seq, MicMS: micMS}

	fail := func(stage string, err error) {
		if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
			return // the kid pressed again or let go of the radio; not an error
		}
		s.log.Error("turn failed", "seq", seq, "stage", stage, "err", err)
		s.send(protocol.Envelope{Type: protocol.TypeNotice, Seq: seq, Code: protocol.NoticeError})
		t.Notice = protocol.NoticeError
		s.record(t)
	}

	s.send(protocol.Envelope{Type: protocol.TypeThinking, Seq: seq})

	heard, err := s.srv.Pipe.STT.Transcribe(ctx, pcm, s.micRate)
	clear(pcm) // the audio has done its job; do not keep it around
	if err != nil {
		fail("stt", err)
		return
	}
	if heard == "" {
		s.send(protocol.Envelope{Type: protocol.TypeNotice, Seq: seq, Code: protocol.NoticeNotHeard})
		t.Notice = protocol.NoticeNotHeard
		s.record(t)
		return
	}
	t.Heard = heard

	r := s.dev.EffectiveRules()
	var remembered []string
	if items, err := s.srv.Store.Remembered(s.dev.ID); err == nil {
		for _, it := range items {
			remembered = append(remembered, it.Text)
		}
	}
	system, err := persona.SystemPrompt(persona.Input{
		Name: s.dev.Kid.Name, Age: s.dev.Kid.Age,
		SteerAround: r.SteerAround, Remembered: remembered,
	})
	if err != nil {
		fail("prompt", err)
		return
	}
	reply, err := s.srv.Pipe.LLM.Reply(ctx, pipeline.ReplyRequest{
		System: system, History: s.srv.history(s.dev.ID, start), Heard: heard,
	})
	if err != nil {
		fail("llm", err)
		return
	}

	checked := guard.Check(reply.Say)
	t.Said, t.Topic, t.Flag = checked.Text, reply.Topic, normalizeFlag(reply.Flag)
	if checked.Blocked {
		t.Flag = persona.FlagNeedsParent
		s.log.Warn("reply blocked by guard", "seq", seq)
	}

	t.LatencyMS = int(time.Since(began).Milliseconds())
	replyDur, err := s.speak(ctx, seq, checked.Text)
	t.ReplyMS = int(replyDur.Milliseconds())
	if err != nil && replyDur == 0 && !errors.Is(ctx.Err(), context.Canceled) {
		fail("tts", err)
		return
	}

	s.record(t)
	s.srv.addHistory(s.dev.ID, start, heard, checked.Text)
	if reply.Remember != "" && !checked.Blocked {
		if err := s.srv.Store.Remember(s.dev.ID, store.Remembered{Time: start, Text: guard.Check(reply.Remember).Text}); err != nil {
			s.log.Error("remember failed", "err", err)
		}
	}
	s.log.Info("turn", "seq", seq, "mic_ms", t.MicMS, "reply_ms", t.ReplyMS,
		"latency_ms", t.LatencyMS, "flag", t.Flag, "truncated", checked.Truncated)
}

// speak streams text as audio to the radio, a little ahead of real time so
// a small device buffer is enough. It returns how much audio was sent.
func (s *session) speak(ctx context.Context, seq int, text string) (time.Duration, error) {
	rate, rc, err := s.srv.Pipe.TTS.Speak(ctx, text)
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	frame := make([]byte, int(frameDuration.Seconds()*float64(rate))*2)
	var (
		sent    int
		started time.Time
		readErr error
	)
	for ctx.Err() == nil {
		n, err := io.ReadFull(rc, frame)
		n &^= 1
		if n > 0 {
			if sent == 0 {
				if err := s.send(protocol.Envelope{Type: protocol.TypeReplyStart, Seq: seq, Codec: protocol.CodecPCM16, Rate: rate}); err != nil {
					return 0, err
				}
				started = time.Now()
			}
			if ahead := audio.Duration(sent, rate) - time.Since(started) - pacingLead; ahead > 0 {
				select {
				case <-time.After(ahead):
				case <-ctx.Done():
				}
				if ctx.Err() != nil {
					break
				}
			}
			if err := s.write(websocket.MessageBinary, frame[:n]); err != nil {
				return audio.Duration(sent, rate), err
			}
			sent += n
		}
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				readErr = err
			}
			break
		}
	}
	if sent > 0 {
		s.send(protocol.Envelope{Type: protocol.TypeReplyEnd, Seq: seq, Cancelled: ctx.Err() != nil})
	}
	return audio.Duration(sent, rate), readErr
}

func (s *session) record(t store.Turn) {
	if err := s.srv.Store.Append(s.dev.ID, s.dev.Location(), t); err != nil {
		s.log.Error("store append failed", "err", err)
	}
}

func (s *session) send(m protocol.Envelope) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.write(websocket.MessageText, b)
}

// write serialises writers. It deliberately uses the session context, not a
// turn context: cancelling a context mid-write closes the whole connection,
// and a cancelled turn must not hang up the radio.
func (s *session) write(typ websocket.MessageType, b []byte) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	ctx, cancel := context.WithTimeout(s.ctx, writeTimeout)
	defer cancel()
	return s.c.Write(ctx, typ, b)
}

func normalizeFlag(f string) string {
	switch f {
	case persona.FlagNeedsParent, persona.FlagSteered, persona.FlagUnclear:
		return f
	}
	return persona.FlagNone
}
