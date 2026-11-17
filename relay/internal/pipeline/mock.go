package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/naveen/roger/relay/internal/audio"
	"github.com/naveen/roger/relay/internal/persona"
)

// Mock is a no-keys, no-network pipeline for bring-up and tests. It "hears"
// a description of the audio, answers with a fixed line, and "speaks" a
// two-note chirp followed by the press played back. Hearing your own voice
// come back through the speaker is the fastest way to prove a new build's
// microphone, speaker and network path all work.
//
// The echo holds the last press in memory, so Mock is for a single developer
// device. Never run it for real kids.
type Mock struct {
	mu       sync.Mutex
	lastPCM  []byte
	lastRate int
}

// Transcribe implements STT.
func (m *Mock) Transcribe(_ context.Context, pcm []byte, rate int) (string, error) {
	m.mu.Lock()
	m.lastPCM, m.lastRate = append([]byte(nil), pcm...), rate
	m.mu.Unlock()
	if audio.RMS(pcm) < 0.002 {
		return "", nil // silence: exercise the not_heard path
	}
	return fmt.Sprintf("(mock) %.1f seconds of audio", audio.Duration(len(pcm), rate).Seconds()), nil
}

// Reply implements LLM.
func (m *Mock) Reply(_ context.Context, req ReplyRequest) (Reply, error) {
	return Reply{
		Say:   "Roger that. This is the mock relay, so here is your own voice coming back. Over.",
		Topic: "radio check",
		Flag:  persona.FlagNone,
	}, nil
}

// Speak implements TTS: chirp, short gap, then the echoed press.
func (m *Mock) Speak(_ context.Context, _ string) (int, io.ReadCloser, error) {
	m.mu.Lock()
	pcm, rate := m.lastPCM, m.lastRate
	m.mu.Unlock()
	if rate == 0 {
		rate = 16000
	}
	var b bytes.Buffer
	b.Write(audio.Tone(880, 90*time.Millisecond, rate, 0.3))
	b.Write(audio.Tone(1320, 120*time.Millisecond, rate, 0.3))
	b.Write(make([]byte, rate/5*2)) // 200 ms of silence
	b.Write(pcm)
	return rate, io.NopCloser(&b), nil
}
