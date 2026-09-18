package qqbot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"wind-agent/internal/domain"
	"wind-agent/internal/session"
)

type Chatter interface {
	Chat(ctx context.Context, in domain.Inbound, t session.Transport) error
}

type Config struct {
	Enable  bool
	AppID   string
	Secret  string
	Address string
	Path    string
}

type Server struct {
	svc  Chatter
	msgr Messenger
	cfg  Config
	mu   sync.Mutex
	seen map[string]time.Time
}

func NewServer(svc Chatter, msgr Messenger, cfg Config) *Server {
	if cfg.Path == "" {
		cfg.Path = "/qqbot"
	}
	if cfg.Address == "" {
		cfg.Address = "0.0.0.0:8443"
	}
	return &Server{
		svc:  svc,
		msgr: msgr,
		cfg:  cfg,
		seen: make(map[string]time.Time),
	}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle(s.cfg.Path, s.handler())
	srv := &http.Server{Addr: s.cfg.Address, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Printf("qqbot webhook listening on %s%s", s.cfg.Address, s.cfg.Path)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	var env envelope
	_ = json.Unmarshal(body, &env)
	if env.Op == opValidation {
		v, err := ParseValidation(body)
		if err != nil {
			http.Error(w, "bad validation", http.StatusBadRequest)
			return
		}
		ack, err := ValidationACK(s.cfg.Secret, v.EventTs, v.PlainToken)
		if err != nil {
			http.Error(w, "sign", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(ack)
		return
	}

	ok, err := Verify(s.cfg.Secret, r.Header, body)
	if err != nil || !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if env.Op == opHeartbeat {
		writeJSON(w, map[string]any{"op": opHeartbeatOK, "d": envHeartbeatSeq(env.D)})
		return
	}

	msg, err := ParseInbound(body)
	if err != nil {
		log.Printf("qqbot parse: %v", err)
		writeJSON(w, map[string]any{"op": opCallbackACK, "d": 1})
		return
	}
	if msg == nil {
		writeJSON(w, map[string]any{"op": opCallbackACK, "d": 0})
		return
	}
	if s.duplicate(msg.Reply.MsgID) {
		writeJSON(w, map[string]any{"op": opCallbackACK, "d": 0})
		return
	}

	go s.handleTurn(*msg)
	writeJSON(w, map[string]any{"op": opCallbackACK, "d": 0})
}

func (s *Server) handleTurn(msg InboundMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t := newTurnTransport(msg.Inbound, msg.Reply, s.msgr)
	defer func() {
		if err := t.Close(); err != nil {
			log.Printf("qqbot reply: %v", err)
		}
	}()
	if err := s.svc.Chat(ctx, msg.Inbound, t); err != nil {
		log.Printf("qqbot chat: %v", err)
		_ = t.Emit(ctx, domain.Event{Type: domain.EventError, Err: err})
	}
}

func (s *Server) duplicate(msgID string) bool {
	if msgID == "" {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts, ok := s.seen[msgID]; ok && now.Sub(ts) < 10*time.Minute {
		return true
	}
	s.seen[msgID] = now
	if len(s.seen) > 4096 {
		for id, ts := range s.seen {
			if now.Sub(ts) > 10*time.Minute {
				delete(s.seen, id)
			}
		}
	}
	return false
}

func envHeartbeatSeq(d json.RawMessage) any {
	var n json.Number
	if err := json.Unmarshal(d, &n); err == nil {
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	return 0
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}
