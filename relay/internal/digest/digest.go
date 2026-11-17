// Package digest builds the daily parent email: what a kid was curious
// about, anything that needs a grown-up, and nothing more. It is a summary
// by design, not a transcript.
package digest

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"

	"github.com/naveen/roger/relay/internal/config"
	"github.com/naveen/roger/relay/internal/persona"
	"github.com/naveen/roger/relay/internal/rules"
	"github.com/naveen/roger/relay/internal/store"
)

//go:embed template.html
var templateHTML string

var tmpl = template.Must(template.New("digest").Parse(templateHTML))

const maxAsked = 5

// Data is what the template renders.
type Data struct {
	Kid            string
	DateLong       string
	Questions      int
	AirtimeMinutes int
	AirtimeLimit   int
	Topics         []string
	HeadsUp        []string
	Asked          []string
	MoreAsked      int
	Remembered     []string
	Steered        int
	Blocked        int
	RetentionDays  int
}

// Build summarises one local day of turns.
func Build(kid string, date time.Time, turns []store.Turn, remembered []store.Remembered, r rules.Rules, loc *time.Location) Data {
	d := Data{
		Kid:           kid,
		DateLong:      date.Format("Monday, January 2"),
		AirtimeLimit:  r.DailyAirtimeMinutes,
		RetentionDays: r.TranscriptRetentionDays,
	}
	seenTopic := map[string]bool{}
	var ms int
	var asked []string
	for _, t := range turns {
		ms += t.MicMS + t.ReplyMS
		if t.Notice == "quiet_hours" || t.Notice == "airtime_exhausted" {
			d.Blocked++
			continue
		}
		if t.Heard == "" || t.Flag == persona.FlagUnclear {
			continue
		}
		d.Questions++
		switch t.Flag {
		case persona.FlagNeedsParent:
			d.HeadsUp = append(d.HeadsUp, t.Heard)
			continue // shown once, in the heads-up box
		case persona.FlagSteered:
			d.Steered++
			continue // counted, not quoted: the parent already knows the topic
		}
		if topic := strings.ToLower(strings.TrimSpace(t.Topic)); topic != "" && !seenTopic[topic] {
			seenTopic[topic] = true
			d.Topics = append(d.Topics, topic)
		}
		asked = append(asked, t.Heard)
	}
	if r.Digest.IncludeQuestions {
		d.Asked = asked
		if len(d.Asked) > maxAsked {
			d.MoreAsked = len(d.Asked) - maxAsked
			d.Asked = d.Asked[:maxAsked]
		}
	}
	d.AirtimeMinutes = (ms + 59999) / 60000
	y, m, day := date.Date()
	for _, rem := range remembered {
		ry, rm, rd := rem.Time.In(loc).Date()
		if ry == y && rm == m && rd == day {
			d.Remembered = append(d.Remembered, rem.Text)
		}
	}
	return d
}

// Render produces the HTML email body.
func Render(d Data) (string, error) {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, d); err != nil {
		return "", err
	}
	return b.String(), nil
}

// Subject is the email subject line.
func Subject(d Data) string {
	if len(d.HeadsUp) > 0 {
		return fmt.Sprintf("%s on the air today, plus one thing worth a chat", d.Kid)
	}
	if d.Questions == 0 {
		return fmt.Sprintf("%s's radio was quiet today", d.Kid)
	}
	return fmt.Sprintf("%s asked Roger %d things today", d.Kid, d.Questions)
}

// Send emails a rendered digest. It uses STARTTLS when the server offers it
// (net/smtp refuses to send credentials over an unencrypted connection).
func Send(s config.SMTP, to, subject, html string) error {
	if !s.Enabled() {
		return fmt.Errorf("digest: SMTP is not configured")
	}
	if strings.ContainsAny(to+subject+s.From, "\r\n") {
		return fmt.Errorf("digest: header injection attempt")
	}
	msg := strings.Join([]string{
		"From: " + s.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=utf-8",
		"",
		html,
	}, "\r\n")
	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Pass, s.Host)
	}
	return smtp.SendMail(fmt.Sprintf("%s:%d", s.Host, s.Port), auth, s.From, []string{to}, []byte(msg))
}
