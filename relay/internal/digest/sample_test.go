package digest

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/naveen/roger/relay/internal/rules"
)

var samplePath = flag.String("sample", "", "write a sample digest to this path")

// TestWriteSample regenerates parent/digest/sample.html (see `make digest-sample`).
func TestWriteSample(t *testing.T) {
	if *samplePath == "" {
		t.Skip("no -sample path given")
	}
	turns, rem, day := sample()
	html, err := Render(Build("Sam", day, turns, rem, rules.Defaults(), time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(*samplePath, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
}
