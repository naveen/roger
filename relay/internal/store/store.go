// Package store keeps what Roger remembers on disk: text only, one JSONL file
// per device per local day, plus a small list of things a kid asked Roger to
// remember. Audio is never written here or anywhere else.
package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Turn is one press and its answer.
type Turn struct {
	Time      time.Time `json:"time"`
	Seq       int       `json:"seq"`
	Heard     string    `json:"heard,omitempty"`
	Said      string    `json:"said,omitempty"`
	Topic     string    `json:"topic,omitempty"`
	Flag      string    `json:"flag,omitempty"`
	Notice    string    `json:"notice,omitempty"`
	MicMS     int       `json:"mic_ms"`
	ReplyMS   int       `json:"reply_ms"`
	LatencyMS int       `json:"latency_ms,omitempty"`
}

// Remembered is something a kid explicitly asked Roger to keep.
type Remembered struct {
	Time time.Time `json:"time"`
	Text string    `json:"text"`
}

// Store is safe for concurrent use.
type Store struct {
	dir string
	mu  sync.Mutex
}

var (
	safeID   = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	dayFile  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.jsonl$`)
	errBadID = errors.New("store: device id must match [a-zA-Z0-9_-]{1,64}")
)

const (
	dateLayout     = "2006-01-02"
	rememberedFile = "remembered.jsonl"
	stateFile      = "state.json"
	maxRemembered  = 50
)

// New creates the data directory if needed.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) deviceDir(device string) (string, error) {
	if !safeID.MatchString(device) {
		return "", errBadID
	}
	d := filepath.Join(s.dir, device)
	return d, os.MkdirAll(d, 0o700)
}

// Append records a turn under the device's local date.
func (s *Store) Append(device string, loc *time.Location, t Turn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deviceDir(device)
	if err != nil {
		return err
	}
	return appendJSONL(filepath.Join(d, t.Time.In(loc).Format(dateLayout)+".jsonl"), t)
}

// Day returns all turns for a local date ("2006-01-02").
func (s *Store) Day(device, date string) ([]Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deviceDir(device)
	if err != nil {
		return nil, err
	}
	if _, err := time.Parse(dateLayout, date); err != nil {
		return nil, err
	}
	var out []Turn
	err = readJSONL(filepath.Join(d, date+".jsonl"), func(line []byte) {
		var t Turn
		if json.Unmarshal(line, &t) == nil {
			out = append(out, t)
		}
	})
	return out, err
}

// AirtimeUsed sums mic and reply audio for a local date.
func (s *Store) AirtimeUsed(device, date string) (time.Duration, error) {
	turns, err := s.Day(device, date)
	if err != nil {
		return 0, err
	}
	var ms int
	for _, t := range turns {
		ms += t.MicMS + t.ReplyMS
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// Remember keeps a note, trimming the list to the newest maxRemembered.
func (s *Store) Remember(device string, r Remembered) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deviceDir(device)
	if err != nil {
		return err
	}
	items := append(s.rememberedLocked(d), r)
	if len(items) > maxRemembered {
		items = items[len(items)-maxRemembered:]
	}
	return writeJSONL(filepath.Join(d, rememberedFile), items)
}

// Remembered returns the kept notes, oldest first.
func (s *Store) Remembered(device string) ([]Remembered, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deviceDir(device)
	if err != nil {
		return nil, err
	}
	return s.rememberedLocked(d), nil
}

func (s *Store) rememberedLocked(dir string) []Remembered {
	var out []Remembered
	readJSONL(filepath.Join(dir, rememberedFile), func(line []byte) {
		var r Remembered
		if json.Unmarshal(line, &r) == nil && r.Text != "" {
			out = append(out, r)
		}
	})
	return out
}

// Prune deletes transcripts older than keepDays and remembered notes older
// than keepRememberedDays, measured from now in loc. It returns how many
// day files were removed.
func (s *Store) Prune(device string, loc *time.Location, now time.Time, keepDays, keepRememberedDays int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deviceDir(device)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return 0, err
	}
	today := now.In(loc)
	cutoff := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -keepDays)
	removed := 0
	for _, e := range entries {
		if !dayFile.MatchString(e.Name()) {
			continue
		}
		day, err := time.Parse(dateLayout, strings.TrimSuffix(e.Name(), ".jsonl"))
		if err != nil || !day.Before(cutoff) {
			continue
		}
		if os.Remove(filepath.Join(d, e.Name())) == nil {
			removed++
		}
	}
	if keepRememberedDays > 0 {
		old := s.rememberedLocked(d)
		var kept []Remembered
		limit := now.AddDate(0, 0, -keepRememberedDays)
		for _, r := range old {
			if r.Time.After(limit) {
				kept = append(kept, r)
			}
		}
		if len(kept) != len(old) {
			if err := writeJSONL(filepath.Join(d, rememberedFile), kept); err != nil {
				return removed, err
			}
		}
	}
	return removed, nil
}

// Erase deletes everything stored for a device.
func (s *Store) Erase(device string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !safeID.MatchString(device) {
		return errBadID
	}
	return os.RemoveAll(filepath.Join(s.dir, device))
}

// DigestSentDate returns the last local date a digest went out for.
func (s *Store) DigestSentDate(device string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deviceDir(device)
	if err != nil {
		return ""
	}
	var st struct {
		DigestSent string `json:"digest_sent"`
	}
	if b, err := os.ReadFile(filepath.Join(d, stateFile)); err == nil {
		json.Unmarshal(b, &st)
	}
	return st.DigestSent
}

// SetDigestSentDate records that the digest for date has gone out.
func (s *Store) SetDigestSentDate(device, date string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deviceDir(device)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]string{"digest_sent": date})
	return os.WriteFile(filepath.Join(d, stateFile), b, 0o600)
}

func appendJSONL(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func writeJSONL[T any](path string, items []T) error {
	var sb strings.Builder
	for _, it := range items {
		b, err := json.Marshal(it)
		if err != nil {
			return err
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSONL(path string, fn func(line []byte)) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) > 0 {
			fn(sc.Bytes())
		}
	}
	return sc.Err()
}
