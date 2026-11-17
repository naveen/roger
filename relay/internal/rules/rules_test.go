package rules

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func at(hhmm string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04", "2026-11-16 "+hhmm)
	return t
}

func TestQuietHoursWrapMidnight(t *testing.T) {
	r := Rules{QuietHours: []Window{{Start: "19:30", End: "07:00"}}}
	cases := map[string]bool{
		"19:29": false, "19:30": true, "23:59": true, "00:00": true,
		"06:59": true, "07:00": false, "12:00": false,
	}
	for hhmm, want := range cases {
		if got := r.InQuietHours(at(hhmm)); got != want {
			t.Errorf("InQuietHours(%s) = %v, want %v", hhmm, got, want)
		}
	}
}

func TestQuietHoursSameDay(t *testing.T) {
	r := Rules{QuietHours: []Window{{Start: "12:00", End: "13:00"}, {Start: "20:00", End: "06:00"}}}
	for hhmm, want := range map[string]bool{"11:59": false, "12:30": true, "13:00": false, "21:00": true} {
		if got := r.InQuietHours(at(hhmm)); got != want {
			t.Errorf("InQuietHours(%s) = %v, want %v", hhmm, got, want)
		}
	}
}

func TestAirtimeLeft(t *testing.T) {
	r := Rules{DailyAirtimeMinutes: 30}
	if got := r.AirtimeLeft(10 * time.Minute); got != 20*time.Minute {
		t.Errorf("got %v", got)
	}
	if got := r.AirtimeLeft(45 * time.Minute); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
	if got := (Rules{}).AirtimeLeft(5 * time.Hour); got <= 0 {
		t.Errorf("zero allowance should mean unlimited, got %v", got)
	}
}

func TestDigestDue(t *testing.T) {
	r := Defaults()
	if r.DigestDue(at("18:59"), "") {
		t.Error("due before send_at")
	}
	if !r.DigestDue(at("19:00"), "2026-11-15") {
		t.Error("not due at send_at")
	}
	if r.DigestDue(at("21:00"), "2026-11-16") {
		t.Error("due twice in one day")
	}
}

func TestValidate(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := Defaults()
	bad.QuietHours = []Window{{Start: "7pm", End: "07:00"}}
	if bad.Validate() == nil {
		t.Error("want error for bad time")
	}
	bad = Defaults()
	bad.VolumeCap = 140
	if bad.Validate() == nil {
		t.Error("want error for volume cap")
	}
}

func TestWithDefaultsNeverMeansForever(t *testing.T) {
	r := Rules{}.WithDefaults()
	if r.TranscriptRetentionDays == 0 || r.RememberedRetentionDays == 0 || r.VolumeCap == 0 {
		t.Errorf("unset fields must fall back to defaults: %+v", r)
	}
}

// The example shipped for parents must stay loadable by the relay.
func TestParentExampleParses(t *testing.T) {
	b, err := os.ReadFile("../../../parent/rules.example.json")
	if err != nil {
		t.Skip("parent/rules.example.json not present:", err)
	}
	var r Rules
	dec := json.NewDecoder(bytesReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
