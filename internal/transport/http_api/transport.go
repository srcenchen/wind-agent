package http_api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"wind-agent/internal/domain"
	"wind-agent/internal/session"
)

// sseTransport 把一次 HTTP 请求包装成 session.Transport。
// HTTP 是「一请求一会话」：Receive 只返回一条入站消息，之后返回 io.EOF。
type sseTransport struct {
	w       http.ResponseWriter
	flusher http.Flusher
	in      domain.Inbound
	sent    bool
}

var _ session.Transport = (*sseTransport)(nil)

func newSSETransport(c *gin.Context, in domain.Inbound) *sseTransport {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	return &sseTransport{
		w:       c.Writer,
		flusher: c.Writer,
		in:      in,
	}
}

func (t *sseTransport) Receive(ctx context.Context) (domain.Inbound, error) {
	if t.sent {
		return domain.Inbound{}, io.EOF
	}
	t.sent = true
	return t.in, nil
}

func (t *sseTransport) Emit(ctx context.Context, ev domain.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data, err := json.Marshal(sseFrame{
		Type:  string(ev.Type),
		Text:  ev.Text,
		Tool:  toolName(ev),
		Error: errText(ev.Err),
	})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(t.w, "data: %s\n\n", data); err != nil {
		return err
	}
	t.flusher.Flush()
	return nil
}

func (t *sseTransport) Close() error { return nil }

type sseFrame struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Tool  string `json:"tool,omitempty"`
	Error string `json:"error,omitempty"`
}

func toolName(ev domain.Event) string {
	if ev.ToolCall == nil {
		return ""
	}
	return ev.ToolCall.Name
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
