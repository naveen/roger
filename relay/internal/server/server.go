// Package server is the relay's WebSocket endpoint: it authenticates a
// radio, enforces the parent's rules, and runs each press through the
// pipeline. It implements the relay side of protocol/PROTOCOL.md.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/naveen/roger/relay/internal/config"
	"github.com/naveen/roger/relay/internal/pipeline"
	"github.com/naveen/roger/relay/internal/protocol"
	"github.com/naveen/roger/relay/internal/store"
)

// Tunables. The press limits are also sent to the device in "ready".
const (
	MaxPress      = 30 * time.Second
	MinPress      = 300 * time.Millisecond
	helloTimeout  = 5 * time.Second
	turnTimeout   = 45 * time.Second
	writeTimeout  = 10 * time.Second
	frameDuration = 40 * time.Millisecond
	pacingLead    = 600 * time.Millisecond // how far ahead of real time audio is sent
	historyTurns  = 6
	historyIdle   = 20 * time.Minute
	noticeSpeech  = time.Minute // at most one spoken notice per device per this long
	readLimit     = 64 * 1024
)

// Server holds everything shared between connections.
type Server struct {
	Devices *config.Devices
	Store   *store.Store
	Pipe    pipeline.Pipeline
	Log     *slog.Logger
	Now     func() time.Time // injectable for tests

	mu    sync.Mutex
	state map[string]*deviceState
}

// deviceState survives reconnects so a dropped Wi-Fi connection does not
// reset the conversation.
type deviceState struct {
	history      []pipeline.Message
	lastTurn     time.Time
	lastSpoken   time.Time // last spoken notice
	lastPruneDay string
	conn         *session
}

// New builds a Server.
func New(devs *config.Devices, st *store.Store, pipe pipeline.Pipeline, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{Devices: devs, Store: st, Pipe: pipe, Log: log, Now: time.Now, state: map[string]*deviceState{}}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /v1/radio", s.handleRadio)
	return mux
}

func (s *Server) handleRadio(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var dev *config.Device
	if token != "" && token != r.Header.Get("Authorization") {
		dev = s.Devices.Authenticate(token)
	}
	if dev == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{protocol.Subprotocol},
		// Origin checks protect cookie-authenticated browser sessions. Radios
		// authenticate with a bearer token and embedded WebSocket clients send
		// arbitrary Origin headers, so the check would only break them.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	if c.Subprotocol() != protocol.Subprotocol {
		c.Close(websocket.StatusPolicyViolation, "subprotocol "+protocol.Subprotocol+" required")
		return
	}
	c.SetReadLimit(readLimit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sess := &session{srv: s, dev: dev, c: c, log: s.Log.With("device", dev.ID)}

	st := s.deviceState(dev.ID)
	s.mu.Lock()
	old := st.conn
	st.conn = sess
	s.mu.Unlock()
	if old != nil {
		// Close waits for the peer's close frame; a radio that dropped off
		// Wi-Fi will never send one, so do not hold up the new connection.
		go old.c.Close(websocket.StatusPolicyViolation, "replaced by a newer connection")
	}

	sess.run(ctx)

	s.mu.Lock()
	if st.conn == sess {
		st.conn = nil
	}
	s.mu.Unlock()
}

func (s *Server) deviceState(id string) *deviceState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state[id]
	if st == nil {
		st = &deviceState{}
		s.state[id] = st
	}
	return st
}

// history returns a copy of the recent conversation, dropping it if the
// radio has been idle long enough that it is a new conversation.
func (s *Server) history(id string, now time.Time) []pipeline.Message {
	st := s.deviceState(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.Sub(st.lastTurn) > historyIdle {
		st.history = nil
	}
	return append([]pipeline.Message(nil), st.history...)
}

func (s *Server) addHistory(id string, now time.Time, heard, said string) {
	st := s.deviceState(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	st.history = append(st.history,
		pipeline.Message{Role: "user", Text: heard},
		pipeline.Message{Role: "assistant", Text: said})
	if extra := len(st.history) - historyTurns*2; extra > 0 {
		st.history = st.history[extra:]
	}
	st.lastTurn = now
}

// allowSpokenNotice rate-limits spoken notices so that mashing the button
// during quiet hours cannot run up a text-to-speech bill.
func (s *Server) allowSpokenNotice(id string, now time.Time) bool {
	st := s.deviceState(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.Sub(st.lastSpoken) < noticeSpeech {
		return false
	}
	st.lastSpoken = now
	return true
}

// pruneOncePerDay applies the retention rules for a device.
func (s *Server) pruneOncePerDay(dev *config.Device, now time.Time) {
	day := now.In(dev.Location()).Format("2006-01-02")
	st := s.deviceState(dev.ID)
	s.mu.Lock()
	done := st.lastPruneDay == day
	st.lastPruneDay = day
	s.mu.Unlock()
	if done {
		return
	}
	r := dev.EffectiveRules()
	n, err := s.Store.Prune(dev.ID, dev.Location(), now, r.TranscriptRetentionDays, r.RememberedRetentionDays)
	if err != nil {
		s.Log.Error("prune failed", "device", dev.ID, "err", err)
	} else if n > 0 {
		s.Log.Info("pruned old transcripts", "device", dev.ID, "days_removed", n)
	}
}
