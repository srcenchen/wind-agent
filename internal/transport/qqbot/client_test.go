package qqbot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRepliesC2CWithAccessToken(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/app/getAppAccessToken":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok-1",
				"expires_in":   "7200",
			})
		case strings.HasPrefix(r.URL.Path, "/v2/users/"):
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "out"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		AppID:  "1903697237",
		Secret: "secret",
		API:    srv.URL,
	})
	err := c.ReplyC2C(context.Background(), "UOPEN", "MSGID", "你好")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "QQBot tok-1" {
		t.Fatalf("auth=%q", gotAuth)
	}
	if gotPath != "/v2/users/UOPEN/messages" {
		t.Fatalf("path=%s", gotPath)
	}
	if !strings.Contains(gotBody, `"content":"你好"`) || !strings.Contains(gotBody, `"msg_id":"MSGID"`) {
		t.Fatalf("body=%s", gotBody)
	}
}

func TestClientRepliesGroup(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/getAppAccessToken" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": "7200"})
			return
		}
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"out"}`))
	}))
	defer srv.Close()
	c := NewClient(ClientConfig{AppID: "id", Secret: "s", API: srv.URL})
	if err := c.ReplyGroup(context.Background(), "G1", "M1", "ok"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/groups/G1/messages" {
		t.Fatalf("path=%s", gotPath)
	}
}
