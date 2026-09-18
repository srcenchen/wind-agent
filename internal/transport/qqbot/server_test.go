package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"wind-agent/internal/domain"
	"wind-agent/internal/session"
)

type fakeChatter struct {
	mu      sync.Mutex
	got     []domain.Inbound
	reply   string
	started chan struct{}
}

func (f *fakeChatter) Chat(ctx context.Context, in domain.Inbound, t session.Transport) error {
	got, err := t.Receive(ctx)
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.got = append(f.got, got)
	f.mu.Unlock()
	if f.started != nil {
		close(f.started)
	}
	if f.reply != "" {
		_ = t.Emit(ctx, domain.Event{Type: domain.EventContent, Text: f.reply})
	}
	return nil
}

func signedRequest(t *testing.T, secret string, body []byte) *http.Request {
	t.Helper()
	ts := "1725442341"
	sig, err := Sign(secret, ts, body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/qqbot", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderSignature, sig)
	req.Header.Set(HeaderTimestamp, ts)
	return req
}

func TestWebhookValidation(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	s := NewServer(&fakeChatter{}, &fakeMessenger{}, Config{
		AppID:  "11111111",
		Secret: secret,
		Path:   "/qqbot",
	})
	body := []byte(`{"d":{"plain_token":"Arq0D5A61EgUu4OxUvOp","event_ts":"1725442341"},"op":13}`)
	req := httptest.NewRequest(http.MethodPost, "/qqbot", bytes.NewReader(body))
	req.Header.Set("User-Agent", "QQBot-Callback")
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		PlainToken string `json:"plain_token"`
		Signature  string `json:"signature"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.PlainToken != "Arq0D5A61EgUu4OxUvOp" {
		t.Fatalf("%+v", got)
	}
	if got.Signature != "87befc99c42c651b3aac0278e71ada338433ae26fcb24307bdc5ad38c1adc2d01bcfcadc0842edac85e85205028a1132afe09280305f13aa6909ffc2d652c706" {
		t.Fatalf("sig=%s", got.Signature)
	}
}

func TestWebhookC2CRepliesAndACKs(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	msgr := &fakeMessenger{}
	chatter := &fakeChatter{reply: "收到了", started: make(chan struct{})}
	s := NewServer(chatter, msgr, Config{AppID: "111", Secret: secret, Path: "/qqbot"})

	body := []byte(`{
		"op":0,
		"t":"C2C_MESSAGE_CREATE",
		"d":{"id":"MSG1","author":{"user_openid":"U1"},"content":"hello"}
	}`)
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, signedRequest(t, secret, body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var ack struct {
		Op int     `json:"op"`
		D  float64 `json:"d"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &ack); err != nil {
		t.Fatal(err)
	}
	if ack.Op != 12 || ack.D != 0 {
		t.Fatalf("ack=%+v", ack)
	}

	select {
	case <-chatter.started:
	case <-time.After(2 * time.Second):
		t.Fatal("chat not started")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(msgr.c2c) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(msgr.c2c) != 1 || msgr.c2c[0].Content != "收到了" || msgr.c2c[0].OpenID != "U1" {
		t.Fatalf("sent %+v", msgr.c2c)
	}
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	s := NewServer(&fakeChatter{}, &fakeMessenger{}, Config{Secret: "DG5g3B4j9X2KOErG", Path: "/qqbot"})
	body := []byte(`{"op":0,"t":"C2C_MESSAGE_CREATE","d":{"id":"m","author":{"user_openid":"u"},"content":"x"}}`)
	req := httptest.NewRequest(http.MethodPost, "/qqbot", bytes.NewReader(body))
	req.Header.Set(HeaderSignature, "00")
	req.Header.Set(HeaderTimestamp, "1")
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestWebhookDedupsSameMsgID(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	msgr := &fakeMessenger{}
	chatter := &fakeChatter{reply: "once"}
	s := NewServer(chatter, msgr, Config{Secret: secret, Path: "/qqbot"})
	body := []byte(`{"op":0,"t":"C2C_MESSAGE_CREATE","d":{"id":"DUP","author":{"user_openid":"U"},"content":"hi"}}`)

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		s.handler().ServeHTTP(rr, signedRequest(t, secret, body))
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d", rr.Code)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		chatter.mu.Lock()
		n := len(chatter.got)
		chatter.mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	chatter.mu.Lock()
	n := len(chatter.got)
	chatter.mu.Unlock()
	if n != 1 {
		t.Fatalf("chat called %d times", n)
	}
}

func TestWebhookHeartbeatACK(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	s := NewServer(&fakeChatter{}, &fakeMessenger{}, Config{Secret: secret, Path: "/qqbot"})
	body := []byte(`{"op":1,"d":42}`)
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, signedRequest(t, secret, body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d %s", rr.Code, rr.Body.String())
	}
	raw, _ := io.ReadAll(rr.Body)
	if !bytes.Contains(raw, []byte(`"op":11`)) {
		t.Fatalf("body=%s", raw)
	}
}
