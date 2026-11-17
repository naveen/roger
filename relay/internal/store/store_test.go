package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendDayAirtime(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Los_Angeles")
	// 06:30 UTC on the 17th is still the 16th in Los Angeles.
	ts := time.Date(2026, 11, 17, 6, 30, 0, 0, time.UTC)
	for i := 1; i <= 3; i++ {
		if err := s.Append("roger-01", loc, Turn{Time: ts, Seq: i, Heard: "why", MicMS: 2000, ReplyMS: 5000}); err != nil {
			t.Fatal(err)
		}
	}
	turns, err := s.Day("roger-01", "2026-11-16")
	if err != nil || len(turns) != 3 {
		t.Fatalf("turns=%d err=%v", len(turns), err)
	}
	used, _ := s.AirtimeUsed("roger-01", "2026-11-16")
	if used != 21*time.Second {
		t.Errorf("airtime = %v, want 21s", used)
	}
	if other, _ := s.Day("roger-01", "2026-11-17"); len(other) != 0 {
		t.Errorf("turn filed under UTC date, not local date")
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	for _, back := range []int{0, 29, 30, 31, 90} {
		ts := now.AddDate(0, 0, -back)
		s.Append("d1", time.UTC, Turn{Time: ts, Heard: "x"})
	}
	s.Remember("d1", Remembered{Time: now.AddDate(0, 0, -100), Text: "old fort idea"})
	s.Remember("d1", Remembered{Time: now.AddDate(0, 0, -5), Text: "fort with a slide exit"})

	removed, err := s.Prune("d1", time.UTC, now, 30, 90)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Errorf("removed %d day files, want 2 (31 and 90 days old)", removed)
	}
	if turns, _ := s.Day("d1", now.AddDate(0, 0, -30).Format(dateLayout)); len(turns) != 1 {
		t.Errorf("day exactly at the retention edge should be kept")
	}
	rem, _ := s.Remembered("d1")
	if len(rem) != 1 || rem[0].Text != "fort with a slide exit" {
		t.Errorf("remembered = %+v", rem)
	}
}

func TestRememberedIsCapped(t *testing.T) {
	s, _ := New(t.TempDir())
	for i := 0; i < maxRemembered+10; i++ {
		s.Remember("d1", Remembered{Time: time.Now(), Text: "idea"})
	}
	rem, _ := s.Remembered("d1")
	if len(rem) != maxRemembered {
		t.Errorf("kept %d, want %d", len(rem), maxRemembered)
	}
}

func TestRejectsPathTraversalAndErases(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	if err := s.Append("../evil", time.UTC, Turn{Time: time.Now()}); err == nil {
		t.Error("want error for unsafe device id")
	}
	s.Append("d1", time.UTC, Turn{Time: time.Now(), Heard: "x"})
	if err := s.Erase("d1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "d1")); !os.IsNotExist(err) {
		t.Error("device dir still exists after Erase")
	}
}

func TestFilesArePrivate(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	now := time.Now()
	s.Append("d1", time.UTC, Turn{Time: now, Heard: "x"})
	fi, err := os.Stat(filepath.Join(dir, "d1", now.UTC().Format(dateLayout)+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}
}
