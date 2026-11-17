// Package persona builds the system prompt and reply schema for Roger.
// The prompt text lives in prompt.md so it can be reviewed and changed
// without reading Go.
package persona

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed prompt.md
var promptText string

var promptTmpl = template.Must(template.New("prompt").Parse(promptText))

// Band is the speaking style for an age range.
type Band struct {
	Name   string
	Length string
	Words  string
}

// BandFor maps an age to a speaking style. Ages outside 4-12 are clamped:
// Roger is not designed for other ages, and the nearest band is the safest
// behaviour if a parent enters one anyway.
func BandFor(age int) Band {
	switch {
	case age <= 5:
		return Band{
			Name:   "4-5",
			Length: "Keep it very short: one or two sentences, about 25 words at most.",
			Words:  "Use the simplest everyday words. Compare new things to things a small child knows: bath time, snacks, pets, the playground.",
		}
	case age <= 8:
		return Band{
			Name:   "6-8",
			Length: "Keep it short: two or three sentences, about 45 words at most.",
			Words:  "Use everyday words. You may introduce one new word if you explain it right away.",
		}
	default:
		return Band{
			Name:   "9-12",
			Length: "Keep it brief: three or four sentences, about 70 words at most.",
			Words:  "You can use real terms and a little more detail, and you can say when scientists are still figuring something out.",
		}
	}
}

// Input is what the prompt needs to know about a kid.
type Input struct {
	Name        string
	Age         int
	SteerAround []string
	Remembered  []string
}

type promptData struct {
	Input
	Band Band
}

// SystemPrompt renders the system prompt for one kid.
func SystemPrompt(in Input) (string, error) {
	if strings.TrimSpace(in.Name) == "" {
		in.Name = "the kid"
	}
	var sb strings.Builder
	err := promptTmpl.Execute(&sb, promptData{Input: in, Band: BandFor(in.Age)})
	return sb.String(), err
}

// Reply flags.
const (
	FlagNone        = "none"
	FlagNeedsParent = "needs_parent"
	FlagSteered     = "steered"
	FlagUnclear     = "unclear"
)

// ReplyToolName is the tool the model must call.
const ReplyToolName = "radio_reply"

// ReplyToolSchema is the JSON Schema for the tool input.
var ReplyToolSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"say": map[string]any{
			"type":        "string",
			"description": "The words Roger speaks aloud. Plain sentences only.",
		},
		"topic": map[string]any{
			"type":        "string",
			"description": "Two to four words naming what the kid was curious about, for the parent's daily summary. Example: \"worms and rain\".",
		},
		"flag": map[string]any{
			"type": "string",
			"enum": []string{FlagNone, FlagNeedsParent, FlagSteered, FlagUnclear},
			"description": "none for ordinary questions; needs_parent when a grown-up should follow up; " +
				"steered when you redirected away from a family rule topic; unclear when you could not understand and asked them to repeat.",
		},
		"remember": map[string]any{
			"type":        "string",
			"description": "Only when the kid asked you to remember something: a short note of what to remember. Otherwise an empty string.",
		},
	},
	"required": []string{"say", "topic", "flag", "remember"},
}
