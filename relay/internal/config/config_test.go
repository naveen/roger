package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "devices.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTokenRoundTrip(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil || !strings.HasPrefix(tok, "rgr_") || len(hash) != 64 {
		t.Fatalf("tok=%q hash=%q err=%v", tok, hash, err)
	}
	ds, err := LoadDevices(write(t, `{"devices":[{"id":"a","token_sha256":"`+hash+`","kid":{"name":"Sam","age":7},"timezone":"America/Los_Angeles"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if d := ds.Authenticate(tok); d == nil || d.ID != "a" {
		t.Error("valid token did not authenticate")
	}
	if ds.Authenticate(tok+"x") != nil || ds.Authenticate("") != nil || ds.Authenticate(hash) != nil {
		t.Error("an invalid token authenticated")
	}
	if r := ds.Devices[0].EffectiveRules(); r.DailyAirtimeMinutes != 30 || len(r.QuietHours) == 0 {
		t.Errorf("a device with no rules should get the defaults, got %+v", r)
	}
}

func TestLoadDevicesRejectsBadInput(t *testing.T) {
	h := strings.Repeat("ab", 32)
	for name, body := range map[string]string{
		"plain token instead of hash": `{"devices":[{"id":"a","token_sha256":"rgr_secret","kid":{"age":7}}]}`,
		"duplicate id":                `{"devices":[{"id":"a","token_sha256":"` + h + `","kid":{"age":7}},{"id":"a","token_sha256":"` + h + `","kid":{"age":7}}]}`,
		"bad timezone":                `{"devices":[{"id":"a","token_sha256":"` + h + `","kid":{"age":7},"timezone":"Mars/Olympus"}]}`,
		"adult age":                   `{"devices":[{"id":"a","token_sha256":"` + h + `","kid":{"age":34}}]}`,
		"bad rules":                   `{"devices":[{"id":"a","token_sha256":"` + h + `","kid":{"age":7},"rules":{"volume_cap":300}}]}`,
	} {
		if _, err := LoadDevices(write(t, body)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// The example shipped in the repo must load.
func TestExampleDevicesFile(t *testing.T) {
	if _, err := LoadDevices("../../devices.example.json"); err != nil {
		t.Fatal(err)
	}
}
