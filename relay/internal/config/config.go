// Package config loads the relay's settings: process settings from the
// environment, and the list of radios it will talk to from a JSON file.
package config

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/naveen/roger/relay/internal/rules"
)

// Config is the process configuration.
type Config struct {
	Addr        string
	DataDir     string
	DevicesFile string
	Provider    string // "live" or "mock"

	AnthropicKey     string
	AnthropicBaseURL string
	LLMModel         string

	AudioKey        string
	AudioBaseURL    string
	STTModel        string
	TTSModel        string
	TTSVoice        string
	TTSInstructions string
	Language        string

	SMTP SMTP
}

// SMTP settings for the daily digest. Host empty means "do not send".
type SMTP struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// Enabled reports whether digests can be emailed.
func (s SMTP) Enabled() bool { return s.Host != "" && s.From != "" }

// FromEnv reads the process configuration.
func FromEnv() (Config, error) {
	c := Config{
		Addr:             env("ROGER_ADDR", ":8080"),
		DataDir:          env("ROGER_DATA_DIR", "./data"),
		DevicesFile:      env("ROGER_DEVICES", "./devices.json"),
		Provider:         env("ROGER_PROVIDER", "live"),
		AnthropicKey:     os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicBaseURL: env("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		LLMModel:         env("ROGER_LLM_MODEL", "claude-haiku-5-5"),
		AudioKey:         os.Getenv("ROGER_AUDIO_API_KEY"),
		AudioBaseURL:     env("ROGER_AUDIO_BASE_URL", "https://api.openai.com/v1"),
		STTModel:         env("ROGER_STT_MODEL", "gpt-4o-mini-transcribe"),
		TTSModel:         env("ROGER_TTS_MODEL", "gpt-4o-mini-tts"),
		TTSVoice:         env("ROGER_TTS_VOICE", "cedar"),
		TTSInstructions:  env("ROGER_TTS_INSTRUCTIONS", "Warm, friendly and unhurried, like a kind radio operator talking to a child. Clear and a little playful."),
		Language:         os.Getenv("ROGER_LANGUAGE"),
		SMTP: SMTP{
			Host: os.Getenv("SMTP_HOST"),
			User: os.Getenv("SMTP_USER"),
			Pass: os.Getenv("SMTP_PASS"),
			From: os.Getenv("SMTP_FROM"),
		},
	}
	port, err := strconv.Atoi(env("SMTP_PORT", "587"))
	if err != nil {
		return c, fmt.Errorf("SMTP_PORT: %w", err)
	}
	c.SMTP.Port = port
	switch c.Provider {
	case "mock":
	case "live":
		if c.AnthropicKey == "" {
			return c, errors.New("ANTHROPIC_API_KEY is required when ROGER_PROVIDER=live (set ROGER_PROVIDER=mock to run without keys)")
		}
		if c.AudioKey == "" && c.AudioBaseURL == "https://api.openai.com/v1" {
			return c, errors.New("ROGER_AUDIO_API_KEY is required for the default audio provider")
		}
	default:
		return c, fmt.Errorf("ROGER_PROVIDER must be live or mock, got %q", c.Provider)
	}
	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Kid is the little the relay knows about who holds the radio. A first name
// or nickname and an age are all it needs; do not add more.
type Kid struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Device is one radio.
type Device struct {
	ID          string       `json:"id"`
	TokenSHA256 string       `json:"token_sha256"`
	Kid         Kid          `json:"kid"`
	Timezone    string       `json:"timezone"`
	ParentEmail string       `json:"parent_email,omitempty"`
	Rules       *rules.Rules `json:"rules,omitempty"`

	loc *time.Location
}

// Location is the device's timezone.
func (d *Device) Location() *time.Location { return d.loc }

// EffectiveRules returns the device's rules with defaults applied.
func (d *Device) EffectiveRules() rules.Rules {
	if d.Rules == nil {
		return rules.Defaults()
	}
	return d.Rules.WithDefaults()
}

// Devices is the set of radios this relay accepts.
type Devices struct {
	Devices []*Device `json:"devices"`
}

// LoadDevices reads and validates the devices file.
func LoadDevices(path string) (*Devices, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ds Devices
	if err := json.Unmarshal(b, &ds); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	seen := map[string]bool{}
	for _, d := range ds.Devices {
		if d.ID == "" || seen[d.ID] {
			return nil, fmt.Errorf("%s: device id %q is empty or duplicated", path, d.ID)
		}
		seen[d.ID] = true
		if raw, err := hex.DecodeString(d.TokenSHA256); err != nil || len(raw) != sha256.Size {
			return nil, fmt.Errorf("%s: device %s: token_sha256 must be 64 hex characters (run `roger-relay token`)", path, d.ID)
		}
		if d.Kid.Age < 1 || d.Kid.Age > 17 {
			return nil, fmt.Errorf("%s: device %s: kid.age %d is not a child's age", path, d.ID, d.Kid.Age)
		}
		if d.Timezone == "" {
			d.Timezone = "UTC"
		}
		loc, err := time.LoadLocation(d.Timezone)
		if err != nil {
			return nil, fmt.Errorf("%s: device %s: timezone: %w", path, d.ID, err)
		}
		d.loc = loc
		if err := d.EffectiveRules().Validate(); err != nil {
			return nil, fmt.Errorf("%s: device %s: rules: %w", path, d.ID, err)
		}
	}
	return &ds, nil
}

// Authenticate returns the device whose token hash matches, or nil. Every
// device is compared so timing does not reveal which ids exist.
func (ds *Devices) Authenticate(token string) *Device {
	sum := sha256.Sum256([]byte(token))
	got := []byte(hex.EncodeToString(sum[:]))
	var match *Device
	for _, d := range ds.Devices {
		if subtle.ConstantTimeCompare(got, []byte(d.TokenSHA256)) == 1 {
			match = d
		}
	}
	return match
}

// ByID finds a device by id.
func (ds *Devices) ByID(id string) *Device {
	for _, d := range ds.Devices {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// NewToken returns a random device token and the hash to store for it.
func NewToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = "rgr_" + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}
