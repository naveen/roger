package audio

import (
	"bytes"
	"testing"
	"time"
)

func TestWAVRoundTrip(t *testing.T) {
	pcm := Tone(440, 250*time.Millisecond, 16000, 0.5)
	got, rate, err := DecodeWAV(bytes.NewReader(EncodeWAV(pcm, 16000)))
	if err != nil || rate != 16000 || !bytes.Equal(got, pcm) {
		t.Fatalf("rate=%d equal=%v err=%v", rate, bytes.Equal(got, pcm), err)
	}
	if _, _, err := DecodeWAV(bytes.NewReader([]byte("not a wav file at all"))); err == nil {
		t.Error("want error for junk input")
	}
}

func TestDurationAndRMS(t *testing.T) {
	if d := Duration(32000, 16000); d != time.Second {
		t.Errorf("Duration = %v", d)
	}
	if RMS(make([]byte, 3200)) != 0 {
		t.Error("silence should have zero RMS")
	}
	if r := RMS(Tone(440, time.Second, 16000, 0.5)); r < 0.3 || r > 0.4 {
		t.Errorf("RMS of a half-scale sine = %.3f, want about 0.35", r)
	}
}
