package digest

import (
	"strings"
	"testing"
	"time"

	"github.com/naveen/roger/relay/internal/rules"
	"github.com/naveen/roger/relay/internal/store"
)

func sample() ([]store.Turn, []store.Remembered, time.Time) {
	day := time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)
	turns := []store.Turn{
		{Time: day, Heard: "Why do worms come out when it rains?", Topic: "Worms and rain", Flag: "none", MicMS: 3000, ReplyMS: 9000},
		{Time: day, Heard: "Can a cloud be heavy?", Topic: "clouds", Flag: "none", MicMS: 2000, ReplyMS: 8000},
		{Time: day, Heard: "What happens when you die?", Topic: "death", Flag: "needs_parent", MicMS: 2000, ReplyMS: 6000},
		{Time: day, Heard: "Tell me a scary story", Topic: "scary story", Flag: "steered", MicMS: 2000, ReplyMS: 5000},
		{Time: day, Heard: "mmph gr", Flag: "unclear", MicMS: 1000, ReplyMS: 2000},
		{Time: day, Notice: "quiet_hours", MicMS: 0},
		{Time: day, Heard: "Do worms have eyes?", Topic: "worms and rain", Flag: "none", MicMS: 2000, ReplyMS: 7000},
	}
	rem := []store.Remembered{
		{Time: day, Text: "A fort with a slide exit"},
		{Time: day.AddDate(0, 0, -3), Text: "older idea"},
	}
	return turns, rem, day
}

func TestBuild(t *testing.T) {
	turns, rem, day := sample()
	d := Build("Sam", day, turns, rem, rules.Defaults(), time.UTC)
	if d.Questions != 5 {
		t.Errorf("Questions = %d, want 5 (unclear and blocked presses do not count)", d.Questions)
	}
	if len(d.Topics) != 2 {
		t.Errorf("Topics = %v, want worms and clouds, deduplicated", d.Topics)
	}
	if len(d.HeadsUp) != 1 || d.Steered != 1 || d.Blocked != 1 {
		t.Errorf("HeadsUp=%v Steered=%d Blocked=%d", d.HeadsUp, d.Steered, d.Blocked)
	}
	if len(d.Asked) != 3 {
		t.Errorf("Asked = %v, want only ordinary questions", d.Asked)
	}
	if len(d.Remembered) != 1 {
		t.Errorf("Remembered = %v, want only today's", d.Remembered)
	}
	if d.AirtimeMinutes != 1 {
		t.Errorf("AirtimeMinutes = %d, want 1 (49s rounds up)", d.AirtimeMinutes)
	}
}

func TestRenderEscapesAndOmits(t *testing.T) {
	turns, rem, day := sample()
	turns[0].Heard = `Why do <b>worms</b> & "bugs" come out?`
	html, err := Render(Build("Sam", day, turns, rem, rules.Defaults(), time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<b>worms</b>") {
		t.Error("kid speech must be HTML-escaped")
	}
	for _, want := range []string{"Sam asked Roger 5 things today", "Worth a conversation", "A fort with a slide exit", "does not keep audio"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(html, "scary story") {
		t.Error("steered topics are counted, not quoted")
	}

	r := rules.Defaults()
	r.Digest.IncludeQuestions = false
	html, _ = Render(Build("Sam", day, turns, rem, r, time.UTC))
	if strings.Contains(html, "Heard on the air") {
		t.Error("questions shown although include_questions is false")
	}
}

func TestQuietDay(t *testing.T) {
	d := Build("Sam", time.Now(), nil, nil, rules.Defaults(), time.UTC)
	html, err := Render(d)
	if err != nil || !strings.Contains(html, "radio was quiet today") {
		t.Errorf("err=%v", err)
	}
	if Subject(d) != "Sam's radio was quiet today" {
		t.Errorf("subject = %q", Subject(d))
	}
}
