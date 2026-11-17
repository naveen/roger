package server

import (
	"context"
	"time"

	"github.com/naveen/roger/relay/internal/config"
	"github.com/naveen/roger/relay/internal/digest"
)

// RunJanitor does the relay's housekeeping once a minute until ctx ends:
// deleting text past its retention period, and sending daily digests.
func (s *Server) RunJanitor(ctx context.Context, smtp config.SMTP) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		s.Housekeep(s.Now(), smtp, digest.Send)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// SendFunc delivers a rendered digest.
type SendFunc func(smtp config.SMTP, to, subject, html string) error

// Housekeep runs one pass. It is exported so tests can drive the clock.
func (s *Server) Housekeep(now time.Time, smtp config.SMTP, send SendFunc) {
	for _, dev := range s.Devices.Devices {
		s.pruneOncePerDay(dev, now)

		r := dev.EffectiveRules()
		local := now.In(dev.Location())
		if !smtp.Enabled() || dev.ParentEmail == "" || !r.DigestDue(local, s.Store.DigestSentDate(dev.ID)) {
			continue
		}
		date := local.Format("2006-01-02")
		turns, err := s.Store.Day(dev.ID, date)
		if err != nil {
			s.Log.Error("digest: read day failed", "device", dev.ID, "err", err)
			continue
		}
		remembered, _ := s.Store.Remembered(dev.ID)
		data := digest.Build(dev.Kid.Name, local, turns, remembered, r, dev.Location())
		html, err := digest.Render(data)
		if err != nil {
			s.Log.Error("digest: render failed", "device", dev.ID, "err", err)
			continue
		}
		if err := send(smtp, dev.ParentEmail, digest.Subject(data), html); err != nil {
			s.Log.Error("digest: send failed", "device", dev.ID, "err", err)
			continue // try again next minute
		}
		s.Store.SetDigestSentDate(dev.ID, date)
		s.Log.Info("digest sent", "device", dev.ID, "date", date)
	}
}
