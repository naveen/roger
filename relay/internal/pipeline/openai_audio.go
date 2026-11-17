package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/naveen/roger/relay/internal/audio"
)

// OpenAIAudio speaks the OpenAI-style audio HTTP API: POST
// {base}/audio/transcriptions and POST {base}/audio/speech. Several hosted
// and self-hosted servers implement the same two endpoints, so the base URL
// is configurable.
type OpenAIAudio struct {
	APIKey       string
	BaseURL      string // e.g. https://api.openai.com/v1
	STTModel     string
	TTSModel     string
	Voice        string
	Instructions string // optional voice direction, ignored by servers that lack it
	Language     string // optional ISO-639-1 hint for transcription
	HTTP         *http.Client
}

// PCMRate is the sample rate of the "pcm" response format on this API.
const PCMRate = 24000

// Transcribe implements STT. The audio is sent as an in-memory WAV.
func (o *OpenAIAudio) Transcribe(ctx context.Context, pcm []byte, rate int) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "press.wav")
	if err != nil {
		return "", err
	}
	fw.Write(audio.EncodeWAV(pcm, rate))
	mw.WriteField("model", o.STTModel)
	mw.WriteField("response_format", "json")
	if o.Language != "" {
		mw.WriteField("language", o.Language)
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	o.auth(req)
	resp, err := o.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stt: %s: %s", resp.Status, truncate(string(raw), 300))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("stt: decode: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

// Speak implements TTS. The response body is raw PCM16LE mono at PCMRate and
// is returned unread so the caller can stream it to the device as it arrives.
func (o *OpenAIAudio) Speak(ctx context.Context, text string) (int, io.ReadCloser, error) {
	payload := map[string]any{
		"model":           o.TTSModel,
		"voice":           o.Voice,
		"input":           text,
		"response_format": "pcm",
	}
	if o.Instructions != "" {
		payload["instructions"] = o.Instructions
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/audio/speech", bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	o.auth(req)
	resp, err := o.client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		return 0, nil, fmt.Errorf("tts: %s: %s", resp.Status, truncate(string(raw), 300))
	}
	return PCMRate, resp.Body, nil
}

func (o *OpenAIAudio) base() string { return strings.TrimRight(o.BaseURL, "/") }

func (o *OpenAIAudio) auth(r *http.Request) {
	if o.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
}

func (o *OpenAIAudio) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return http.DefaultClient
}
