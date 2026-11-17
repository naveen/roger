package pipeline

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/naveen/roger/relay/internal/audio"
	"github.com/naveen/roger/relay/internal/persona"
)

func TestAnthropicRequestAndResponse(t *testing.T) {
	var got map[string]any
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("bad request: %s %v", r.URL.Path, r.Header)
		}
		json.NewDecoder(r.Body).Decode(&got)
		io.WriteString(w, `{"content":[{"type":"tool_use","name":"radio_reply","input":{"say":"Roger that.","topic":"worms","flag":"none","remember":""}}]}`)
	}))
	defer hs.Close()

	a := &Anthropic{APIKey: "k", Model: "m", BaseURL: hs.URL}
	rep, err := a.Reply(context.Background(), ReplyRequest{
		System:  "sys",
		History: []Message{{Role: "user", Text: "hi"}, {Role: "assistant", Text: "hello"}},
		Heard:   "why worms",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Say != "Roger that." || rep.Topic != "worms" {
		t.Errorf("reply = %+v", rep)
	}
	if got["model"] != "m" || got["system"] != "sys" || len(got["messages"].([]any)) != 3 {
		t.Errorf("request = %v", got)
	}
	choice := got["tool_choice"].(map[string]any)
	if choice["type"] != "tool" || choice["name"] != persona.ReplyToolName {
		t.Errorf("tool_choice = %v, want the reply tool forced", choice)
	}
}

func TestAnthropicErrors(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("x-api-key"), "bad") {
			http.Error(w, `{"error":{"message":"invalid key"}}`, http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"content":[{"type":"text","text":"free text, no tool call"}]}`)
	}))
	defer hs.Close()
	if _, err := (&Anthropic{APIKey: "bad", BaseURL: hs.URL}).Reply(context.Background(), ReplyRequest{Heard: "x"}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("want 401 error, got %v", err)
	}
	// Free text must never be spoken as if it were a structured reply.
	if _, err := (&Anthropic{APIKey: "k", BaseURL: hs.URL}).Reply(context.Background(), ReplyRequest{Heard: "x"}); err == nil {
		t.Error("want error when the model did not call the tool")
	}
}

func TestOpenAIAudio(t *testing.T) {
	pcm := audio.Tone(440, 500*time.Millisecond, 16000, 0.3)
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/v1/audio/transcriptions":
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			got, rate, err := audio.DecodeWAV(f)
			if err != nil || rate != 16000 || len(got) != len(pcm) {
				t.Errorf("uploaded WAV: rate=%d len=%d err=%v", rate, len(got), err)
			}
			if r.FormValue("model") != "stt-model" {
				t.Errorf("model = %q", r.FormValue("model"))
			}
			io.WriteString(w, `{"text":" why do worms come out "}`)
		case "/v1/audio/speech":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["response_format"] != "pcm" || body["voice"] != "v" || body["input"] != "Roger that." {
				t.Errorf("speech body = %v", body)
			}
			w.Write(audio.Tone(600, 200*time.Millisecond, PCMRate, 0.2))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer hs.Close()

	o := &OpenAIAudio{APIKey: "k", BaseURL: hs.URL + "/v1/", STTModel: "stt-model", TTSModel: "tts-model", Voice: "v"}
	text, err := o.Transcribe(context.Background(), pcm, 16000)
	if err != nil || text != "why do worms come out" {
		t.Errorf("text=%q err=%v", text, err)
	}
	rate, rc, err := o.Speak(context.Background(), "Roger that.")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if rate != 24000 || audio.Duration(len(b), rate) != 200*time.Millisecond {
		t.Errorf("rate=%d dur=%v", rate, audio.Duration(len(b), rate))
	}
}

func TestMockEchoesThePress(t *testing.T) {
	m := &Mock{}
	pcm := audio.Tone(440, time.Second, 16000, 0.3)
	heard, _ := m.Transcribe(context.Background(), pcm, 16000)
	if heard == "" {
		t.Fatal("mock should hear a tone")
	}
	if silent, _ := (&Mock{}).Transcribe(context.Background(), make([]byte, 32000), 16000); silent != "" {
		t.Error("mock should not hear silence")
	}
	rate, rc, _ := m.Speak(context.Background(), "x")
	b, _ := io.ReadAll(rc)
	if rate != 16000 || len(b) <= len(pcm) {
		t.Errorf("rate=%d len=%d, want chirp plus the %d-byte press", rate, len(b), len(pcm))
	}
}
