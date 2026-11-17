// Command roger-sim pretends to be a Roger radio. It makes one press against
// a relay, prints the message timeline, and saves what came back, so the
// relay can be developed and tested with no hardware.
//
//	roger-sim -token rgr_... -wav question.wav -out reply.wav
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/coder/websocket"

	"github.com/naveen/roger/relay/internal/audio"
	"github.com/naveen/roger/relay/internal/protocol"
)

func main() {
	url := flag.String("url", "ws://localhost:8080/v1/radio", "relay WebSocket URL")
	token := flag.String("token", os.Getenv("ROGER_TOKEN"), "device token (or set ROGER_TOKEN)")
	wav := flag.String("wav", "", "PCM16 mono WAV to send as the press (default: a 1.5 s test tone)")
	out := flag.String("out", "reply.wav", "where to save the reply audio")
	fast := flag.Bool("fast", false, "send the press as fast as possible instead of in real time")
	flag.Parse()
	if err := run(*url, *token, *wav, *out, *fast); err != nil {
		fmt.Fprintln(os.Stderr, "roger-sim:", err)
		os.Exit(1)
	}
}

func run(url, token, wavPath, outPath string, fast bool) error {
	if token == "" {
		return errors.New("a device token is required (-token or ROGER_TOKEN)")
	}
	pcm, rate := audio.Tone(440, 1500*time.Millisecond, 16000, 0.3), 16000
	if wavPath != "" {
		f, err := os.Open(wavPath)
		if err != nil {
			return err
		}
		pcm, rate, err = audio.DecodeWAV(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", wavPath, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		Subprotocols: []string{protocol.Subprotocol},
		HTTPHeader:   http.Header{"Authorization": {"Bearer " + token}},
	})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("connect: %s", resp.Status)
		}
		return fmt.Errorf("connect: %w", err)
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)

	t0 := time.Now()
	stamp := func(format string, a ...any) {
		fmt.Printf("%7.2fs  %s\n", time.Since(t0).Seconds(), fmt.Sprintf(format, a...))
	}
	send := func(m protocol.Envelope) error {
		b, _ := json.Marshal(m)
		return c.Write(ctx, websocket.MessageText, b)
	}

	if err := send(protocol.Envelope{Type: protocol.TypeHello, Proto: protocol.Subprotocol, FW: "sim", Board: "roger-sim",
		Mic: &protocol.AudioSpec{Codec: protocol.CodecPCM16, Rate: rate}}); err != nil {
		return err
	}
	if m, _, err := readText(ctx, c); err != nil || m.Type != protocol.TypeReady {
		return fmt.Errorf("expected ready, got %q (%v)", m.Type, err)
	} else {
		stamp("ready           max_press=%dms volume_cap=%d", m.Limits.MaxPressMS, m.Limits.VolumeCap)
	}

	const seq = 1
	stamp("ptt.down        sending %.1fs of audio", audio.Duration(len(pcm), rate).Seconds())
	if err := send(protocol.Envelope{Type: protocol.TypePTTDown, Seq: seq}); err != nil {
		return err
	}
	frame := rate / 25 * 2 // 40 ms
	for off := 0; off < len(pcm); off += frame {
		end := min(off+frame, len(pcm))
		if err := c.Write(ctx, websocket.MessageBinary, pcm[off:end]); err != nil {
			return err
		}
		if !fast {
			time.Sleep(40 * time.Millisecond)
		}
	}
	if err := send(protocol.Envelope{Type: protocol.TypePTTUp, Seq: seq}); err != nil {
		return err
	}
	up := time.Now()
	stamp("ptt.up")

	var reply []byte
	replyRate := 0
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		if typ == websocket.MessageBinary {
			reply = append(reply, data...)
			continue
		}
		var m protocol.Envelope
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		switch m.Type {
		case protocol.TypeThinking:
			stamp("thinking        (the radio plays static now)")
		case protocol.TypeReplyStart:
			replyRate = m.Rate
			stamp("reply.start     %s @ %d Hz, %.2fs after ptt.up", m.Codec, m.Rate, time.Since(up).Seconds())
		case protocol.TypeReplyEnd:
			stamp("reply.end       %.1fs of audio", audio.Duration(len(reply), replyRate).Seconds())
			if err := os.WriteFile(outPath, audio.EncodeWAV(reply, replyRate), 0o600); err != nil {
				return err
			}
			fmt.Println("saved", outPath)
			c.Close(websocket.StatusNormalClosure, "")
			return nil
		case protocol.TypeNotice:
			stamp("notice          code=%s speech=%v", m.Code, m.Speech)
			if !m.Speech {
				c.Close(websocket.StatusNormalClosure, "")
				if m.Code == protocol.NoticeError {
					return errors.New("relay reported an error for this press; check its log")
				}
				return nil
			}
		}
	}
}

func readText(ctx context.Context, c *websocket.Conn) (protocol.Envelope, []byte, error) {
	var m protocol.Envelope
	typ, data, err := c.Read(ctx)
	if err != nil {
		return m, nil, err
	}
	if typ != websocket.MessageText {
		return m, data, errors.New("expected a text frame")
	}
	return m, data, json.Unmarshal(data, &m)
}
