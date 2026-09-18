package qqbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"wind-agent/internal/domain"
)

type Gateway struct {
	svc    Chatter
	client *Client
	cfg    Config
	mu     sync.Mutex
	seen   map[string]time.Time
}

func NewGateway(svc Chatter, client *Client, cfg Config) *Gateway {
	return &Gateway{
		svc:    svc,
		client: client,
		cfg:    cfg,
		seen:   make(map[string]time.Time),
	}
}

func (g *Gateway) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		log.Printf("qqbot connecting websocket gateway")
		err := g.connectOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("qqbot gateway disconnected: %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (g *Gateway) connectOnce(ctx context.Context) error {
	url, err := g.client.GatewayURL(ctx)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("dial gateway: %w", err)
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	_, helloRaw, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read hello: %w", err)
	}
	intervalMS, err := ParseHello(helloRaw)
	if err != nil {
		return err
	}

	token, err := g.client.AccessToken(ctx)
	if err != nil {
		return err
	}
	identify := map[string]any{
		"op": opIdentify,
		"d": map[string]any{
			"token":   "QQBot " + token,
			"intents": IntentGroupAndC2C,
			"shard":   []int{0, 1},
		},
	}
	if err := writeJSON(identify); err != nil {
		return fmt.Errorf("identify: %w", err)
	}

	var seqMu sync.Mutex
	var lastSeq any
	stopHB := make(chan struct{})
	defer close(stopHB)
	go func() {
		ticker := time.NewTicker(time.Duration(intervalMS) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopHB:
				return
			case <-ticker.C:
				seqMu.Lock()
				d := lastSeq
				seqMu.Unlock()
				_ = writeJSON(map[string]any{"op": opHeartbeat, "d": d})
			}
		}
	}()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			log.Printf("qqbot ws bad payload: %v", err)
			continue
		}
		if env.S != nil {
			seqMu.Lock()
			lastSeq = *env.S
			seqMu.Unlock()
		}
		switch env.Op {
		case opHeartbeatOK:
			continue
		case opReconnect, opInvalidSess:
			return fmt.Errorf("gateway op=%d", env.Op)
		case opHeartbeat:
			seqMu.Lock()
			d := lastSeq
			seqMu.Unlock()
			_ = writeJSON(map[string]any{"op": opHeartbeat, "d": d})
		case opDispatch:
			if env.T == eventReady {
				if id, err := ParseReadySessionID(raw); err == nil {
					log.Printf("qqbot websocket ready session=%s", id)
				}
				continue
			}
			msg, err := ParseInbound(raw)
			if err != nil {
				log.Printf("qqbot parse: %v", err)
				continue
			}
			if msg == nil || g.duplicate(msg.Reply.MsgID) {
				continue
			}
			go g.handleTurn(*msg)
		}
	}
}

func (g *Gateway) handleTurn(msg InboundMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t := newTurnTransport(msg.Inbound, msg.Reply, g.client)
	defer func() {
		if err := t.Close(); err != nil {
			log.Printf("qqbot reply: %v", err)
		}
	}()
	if err := g.svc.Chat(ctx, msg.Inbound, t); err != nil {
		log.Printf("qqbot chat: %v", err)
		_ = t.Emit(ctx, domain.Event{Type: domain.EventError, Err: err})
	}
}

func (g *Gateway) duplicate(msgID string) bool {
	if msgID == "" {
		return false
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if ts, ok := g.seen[msgID]; ok && now.Sub(ts) < 10*time.Minute {
		return true
	}
	g.seen[msgID] = now
	if len(g.seen) > 4096 {
		for id, ts := range g.seen {
			if now.Sub(ts) > 10*time.Minute {
				delete(g.seen, id)
			}
		}
	}
	return false
}
