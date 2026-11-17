package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/naveen/roger/relay/internal/persona"
)

// Anthropic calls the Claude Messages API. The reply is forced through a
// tool call so the relay always gets structured fields, never free text it
// would have to parse.
type Anthropic struct {
	APIKey  string
	Model   string
	BaseURL string // default https://api.anthropic.com
	HTTP    *http.Client
}

type anthropicContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Reply implements LLM.
func (a *Anthropic) Reply(ctx context.Context, req ReplyRequest) (Reply, error) {
	msgs := make([]anthropicMessage, 0, len(req.History)+1)
	for _, m := range req.History {
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Text})
	}
	msgs = append(msgs, anthropicMessage{Role: "user", Content: req.Heard})

	body, _ := json.Marshal(map[string]any{
		"model":      a.Model,
		"max_tokens": 400,
		"system":     req.System,
		"messages":   msgs,
		"tools": []map[string]any{{
			"name":         persona.ReplyToolName,
			"description":  "Speak a reply to the kid over the radio.",
			"input_schema": persona.ReplyToolSchema,
		}},
		"tool_choice": map[string]any{"type": "tool", "name": persona.ReplyToolName},
	})

	base := strings.TrimRight(a.BaseURL, "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Reply{}, err
	}
	hreq.Header.Set("content-type", "application/json")
	hreq.Header.Set("x-api-key", a.APIKey)
	hreq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client().Do(hreq)
	if err != nil {
		return Reply{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Reply{}, fmt.Errorf("anthropic: %s: %s", resp.Status, truncate(string(raw), 300))
	}
	var out struct {
		Content []anthropicContent `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Reply{}, fmt.Errorf("anthropic: decode: %w", err)
	}
	for _, c := range out.Content {
		if c.Type == "tool_use" && c.Name == persona.ReplyToolName {
			var in struct {
				Say      string `json:"say"`
				Topic    string `json:"topic"`
				Flag     string `json:"flag"`
				Remember string `json:"remember"`
			}
			if err := json.Unmarshal(c.Input, &in); err != nil {
				return Reply{}, fmt.Errorf("anthropic: tool input: %w", err)
			}
			return Reply{Say: in.Say, Topic: in.Topic, Flag: in.Flag, Remember: in.Remember}, nil
		}
	}
	return Reply{}, errors.New("anthropic: response had no radio_reply tool call")
}

func (a *Anthropic) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return http.DefaultClient
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
