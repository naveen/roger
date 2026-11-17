package persona

import (
	"strings"
	"testing"
)

func TestSystemPrompt(t *testing.T) {
	p, err := SystemPrompt(Input{Name: "Sam", Age: 7, SteerAround: []string{"scary stories"}, Remembered: []string{"fort with a slide exit"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Sam, who is 7 years old", "about 45 words", "scary stories", "fort with a slide exit", "You are an AI", ReplyToolName} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "{{") {
		t.Error("unrendered template action in prompt")
	}
}

func TestOptionalSectionsDisappear(t *testing.T) {
	p, _ := SystemPrompt(Input{Name: "Ada", Age: 10})
	if strings.Contains(p, "extra rules") || strings.Contains(p, "asked you to remember before") {
		t.Error("empty optional sections should not render")
	}
}

func TestBands(t *testing.T) {
	for age, want := range map[int]string{3: "4-5", 5: "4-5", 6: "6-8", 8: "6-8", 9: "9-12", 15: "9-12"} {
		if got := BandFor(age).Name; got != want {
			t.Errorf("BandFor(%d) = %s, want %s", age, got, want)
		}
	}
}
