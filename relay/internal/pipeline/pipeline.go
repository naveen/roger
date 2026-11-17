// Package pipeline is the chain a press goes through: speech to text, text to
// a reply, reply to speech. Each stage is an interface so providers can be
// swapped, and so the text in the middle can be checked before anything is
// spoken to a child.
package pipeline

import (
	"context"
	"io"
)

// STT turns one press of PCM16LE mono audio into text.
type STT interface {
	Transcribe(ctx context.Context, pcm []byte, rate int) (string, error)
}

// Message is one prior exchange line.
type Message struct {
	Role string // "user" or "assistant"
	Text string
}

// ReplyRequest is everything the model sees.
type ReplyRequest struct {
	System  string
	History []Message
	Heard   string
}

// Reply is the model's structured answer.
type Reply struct {
	Say      string
	Topic    string
	Flag     string
	Remember string
}

// LLM produces Roger's reply.
type LLM interface {
	Reply(ctx context.Context, req ReplyRequest) (Reply, error)
}

// TTS turns text into a stream of PCM16LE mono audio at the returned rate.
type TTS interface {
	Speak(ctx context.Context, text string) (rate int, audio io.ReadCloser, err error)
}

// Pipeline bundles the three stages.
type Pipeline struct {
	STT STT
	LLM LLM
	TTS TTS
}
