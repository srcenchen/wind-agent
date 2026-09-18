package qqbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"wind-agent/internal/domain"
)

func TestClientGatewayURL(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-gw", "expires_in": "7200"})
		case "/gateway":
			gotAuth = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode(map[string]string{"url": "wss://api.bot.qq.com/websocket"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := NewClient(ClientConfig{AppID: "id", Secret: "s", API: srv.URL})
	u, err := c.GatewayURL(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u != "wss://api.bot.qq.com/websocket" {
		t.Fatalf("url=%s", u)
	}
	if gotAuth != "QQBot tok-gw" {
		t.Fatalf("auth=%q", gotAuth)
	}
}

func TestParseHelloInterval(t *testing.T) {
	raw := []byte(`{"op":10,"d":{"heartbeat_interval":45000}}`)
	ms, err := ParseHello(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ms != 45000 {
		t.Fatalf("ms=%d", ms)
	}
}

func TestParseReadySessionID(t *testing.T) {
	raw := []byte(`{"op":0,"t":"READY","s":1,"d":{"session_id":"sess-1"}}`)
	id, err := ParseReadySessionID(raw)
	if err != nil {
		t.Fatal(err)
	}
	if id != "sess-1" {
		t.Fatalf("id=%s", id)
	}
}

func TestIntentIncludesGroupAndC2C(t *testing.T) {
	if IntentGroupAndC2C != 1<<25 {
		t.Fatalf("intent=%d", IntentGroupAndC2C)
	}
}

func TestGatewayConnectsIdentifiesAndHandlesC2C(t *testing.T) {
	var (
		mu        sync.Mutex
		identify  map[string]any
		heartbeat map[string]any
	)
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"op": 10, "d": map[string]any{"heartbeat_interval": 200}})
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var env map[string]any
		_ = json.Unmarshal(data, &env)
		mu.Lock()
		identify = env
		mu.Unlock()
		_ = conn.WriteJSON(map[string]any{
			"op": 0, "t": "READY", "s": 1,
			"d": map[string]any{"session_id": "sess-ok"},
		})
		_ = conn.WriteJSON(map[string]any{
			"op": 0, "t": "C2C_MESSAGE_CREATE", "s": 2,
			"d": map[string]any{
				"id":      "MSG1",
				"content": "hello",
				"author":  map[string]any{"user_openid": "U1"},
			},
		})
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			_, data, err := conn.ReadMessage()
			if err != nil {
				continue
			}
			var hb map[string]any
			if json.Unmarshal(data, &hb) == nil {
				if op, _ := hb["op"].(float64); op == 1 {
					mu.Lock()
					heartbeat = hb
					mu.Unlock()
					return
				}
			}
		}
	}))
	defer wsSrv.Close()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-ws", "expires_in": "7200"})
		case "/gateway":
			wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http")
			_ = json.NewEncoder(w).Encode(map[string]string{"url": wsURL})
		case "/v2/users/U1/messages":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"out"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer httpSrv.Close()

	client := NewClient(ClientConfig{AppID: "1903697237", Secret: "secret", API: httpSrv.URL})
	chatter := &fakeChatter{reply: "pong", started: make(chan struct{})}
	gw := NewGateway(chatter, client, Config{AppID: "1903697237", Secret: "secret"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- gw.Run(ctx) }()

	select {
	case <-chatter.started:
	case err := <-errCh:
		t.Fatalf("gateway stopped: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("chat not started")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := identify != nil && heartbeat != nil
		mu.Unlock()
		chatter.mu.Lock()
		n := len(chatter.got)
		chatter.mu.Unlock()
		if ok && n == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if identify == nil {
		t.Fatal("no identify")
	}
	if op, _ := identify["op"].(float64); op != 2 {
		t.Fatalf("identify op=%v", identify["op"])
	}
	d, _ := identify["d"].(map[string]any)
	if tok, _ := d["token"].(string); tok != "QQBot tok-ws" {
		t.Fatalf("token=%v", d["token"])
	}
	if intents, _ := d["intents"].(float64); int(intents) != IntentGroupAndC2C {
		t.Fatalf("intents=%v", d["intents"])
	}
	if heartbeat == nil {
		t.Fatal("no heartbeat")
	}
	chatter.mu.Lock()
	defer chatter.mu.Unlock()
	if len(chatter.got) != 1 || chatter.got[0].SessionId != "qq:c2c:U1" || chatter.got[0].Content != "hello" {
		t.Fatalf("got %+v", chatter.got)
	}
	_ = domain.Inbound{}
}
