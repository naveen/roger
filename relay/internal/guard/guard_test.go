package guard

import (
	"strings"
	"testing"
)

func TestCleansMarkupAndLinks(t *testing.T) {
	r := Check("**Worms** breathe through their skin! See https://example.com/worms for more.")
	if r.Blocked || strings.ContainsAny(r.Text, "*#") || strings.Contains(r.Text, "http") {
		t.Errorf("got %+v", r)
	}
	if !strings.HasPrefix(r.Text, "Worms breathe") {
		t.Errorf("got %q", r.Text)
	}
}

func TestBlocksAndFallsBack(t *testing.T) {
	for _, s := range []string{"", "   ", "well shit, that's a tough one"} {
		if r := Check(s); !r.Blocked || r.Text != Fallback {
			t.Errorf("Check(%q) = %+v, want fallback", s, r)
		}
	}
	// Substrings of ordinary words must not trip it.
	if r := Check("The Scunthorpe class went to see the shiitake farm."); r.Blocked {
		t.Errorf("false positive: %+v", r)
	}
}

func TestTruncatesAtSentence(t *testing.T) {
	long := strings.Repeat("Clouds are made of tiny drops of water. ", 40)
	r := Check(long)
	if !r.Truncated || len(r.Text) > MaxSpokenChars || !strings.HasSuffix(r.Text, ".") {
		t.Errorf("len=%d truncated=%v tail=%q", len(r.Text), r.Truncated, r.Text[len(r.Text)-10:])
	}
}
