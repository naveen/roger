// Package rules holds the parent-set rules for one device and the checks the
// relay runs before it lets a press through. The JSON shape is specified by
// parent/rules.schema.json at the repo root.
package rules

import (
	"fmt"
	"time"
)

// Rules is one device's parent-set configuration.
type Rules struct {
	QuietHours              []Window `json:"quiet_hours"`
	DailyAirtimeMinutes     int      `json:"daily_airtime_minutes"`
	VolumeCap               int      `json:"volume_cap"`
	SteerAround             []string `json:"steer_around"`
	TranscriptRetentionDays int      `json:"transcript_retention_days"`
	RememberedRetentionDays int      `json:"remembered_retention_days"`
	Digest                  Digest   `json:"digest"`
}

// Window is a daily local-time window, "HH:MM" to "HH:MM". It may wrap
// midnight (start 19:30, end 07:00).
type Window struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Digest controls the daily parent email.
type Digest struct {
	Enabled          bool   `json:"enabled"`
	SendAt           string `json:"send_at"`
	IncludeQuestions bool   `json:"include_questions"`
}

// Defaults are deliberately conservative; a device with no rules block gets
// these.
func Defaults() Rules {
	return Rules{
		QuietHours:              []Window{{Start: "19:30", End: "07:00"}},
		DailyAirtimeMinutes:     30,
		VolumeCap:               70,
		TranscriptRetentionDays: 30,
		RememberedRetentionDays: 90,
		Digest:                  Digest{Enabled: true, SendAt: "19:00", IncludeQuestions: true},
	}
}

// WithDefaults fills zero-valued retention and cap fields so that a partial
// rules block cannot accidentally mean "keep forever" or "no volume cap".
func (r Rules) WithDefaults() Rules {
	d := Defaults()
	if r.TranscriptRetentionDays <= 0 {
		r.TranscriptRetentionDays = d.TranscriptRetentionDays
	}
	if r.RememberedRetentionDays <= 0 {
		r.RememberedRetentionDays = d.RememberedRetentionDays
	}
	if r.VolumeCap <= 0 {
		r.VolumeCap = d.VolumeCap
	}
	if r.Digest.SendAt == "" {
		r.Digest.SendAt = d.Digest.SendAt
	}
	return r
}

// Validate reports the first problem with a rules block.
func (r Rules) Validate() error {
	for i, w := range r.QuietHours {
		if _, err := parseHHMM(w.Start); err != nil {
			return fmt.Errorf("quiet_hours[%d].start: %w", i, err)
		}
		if _, err := parseHHMM(w.End); err != nil {
			return fmt.Errorf("quiet_hours[%d].end: %w", i, err)
		}
	}
	if r.DailyAirtimeMinutes < 0 || r.DailyAirtimeMinutes > 24*60 {
		return fmt.Errorf("daily_airtime_minutes: %d out of range", r.DailyAirtimeMinutes)
	}
	if r.VolumeCap < 0 || r.VolumeCap > 100 {
		return fmt.Errorf("volume_cap: %d out of range 0-100", r.VolumeCap)
	}
	if r.TranscriptRetentionDays < 0 || r.TranscriptRetentionDays > 365 {
		return fmt.Errorf("transcript_retention_days: %d out of range 1-365", r.TranscriptRetentionDays)
	}
	if r.RememberedRetentionDays < 0 || r.RememberedRetentionDays > 365 {
		return fmt.Errorf("remembered_retention_days: %d out of range 1-365", r.RememberedRetentionDays)
	}
	if r.Digest.SendAt != "" {
		if _, err := parseHHMM(r.Digest.SendAt); err != nil {
			return fmt.Errorf("digest.send_at: %w", err)
		}
	}
	return nil
}

// InQuietHours reports whether local time t falls in any quiet window.
// t must already be in the device's timezone.
func (r Rules) InQuietHours(t time.Time) bool {
	now := t.Hour()*60 + t.Minute()
	for _, w := range r.QuietHours {
		s, err1 := parseHHMM(w.Start)
		e, err2 := parseHHMM(w.End)
		if err1 != nil || err2 != nil || s == e {
			continue
		}
		if s < e {
			if now >= s && now < e {
				return true
			}
		} else if now >= s || now < e { // wraps midnight
			return true
		}
	}
	return false
}

// AirtimeLeft returns how much of today's allowance remains. A zero
// allowance means unlimited and returns a large positive duration.
func (r Rules) AirtimeLeft(used time.Duration) time.Duration {
	if r.DailyAirtimeMinutes == 0 {
		return 24 * time.Hour
	}
	left := time.Duration(r.DailyAirtimeMinutes)*time.Minute - used
	if left < 0 {
		return 0
	}
	return left
}

// DigestDue reports whether the digest should go out at local time t, given
// the local date ("2006-01-02") it was last sent for.
func (r Rules) DigestDue(t time.Time, lastSentDate string) bool {
	if !r.Digest.Enabled {
		return false
	}
	at, err := parseHHMM(r.Digest.SendAt)
	if err != nil {
		return false
	}
	if t.Format("2006-01-02") == lastSentDate {
		return false
	}
	return t.Hour()*60+t.Minute() >= at
}

func parseHHMM(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("%q is not HH:MM", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}
