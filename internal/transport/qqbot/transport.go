package qqbot

import (
	"context"
	"errors"
	"io"
	"strings"

	"wind-agent/internal/domain"
	"wind-agent/internal/session"
)

type Messenger interface {
	ReplyC2C(ctx context.Context, userOpenID, msgID, content string) error
	ReplyGroup(ctx context.Context, groupOpenID, msgID, content string) error
}

type turnTransport struct {
	in     domain.Inbound
	reply  ReplyTarget
	msgr   Messenger
	got    bool
	buf    strings.Builder
	errMsg string
}

func newTurnTransport(in domain.Inbound, reply ReplyTarget, msgr Messenger) *turnTransport {
	return &turnTransport{in: in, reply: reply, msgr: msgr}
}

var _ session.Transport = (*turnTransport)(nil)

func (t *turnTransport) Receive(_ context.Context) (domain.Inbound, error) {
	if t.got {
		return domain.Inbound{}, io.EOF
	}
	t.got = true
	return t.in, nil
}

func (t *turnTransport) Emit(_ context.Context, ev domain.Event) error {
	switch ev.Type {
	case domain.EventContent:
		t.buf.WriteString(ev.Text)
	case domain.EventError:
		if ev.Err != nil {
			t.errMsg = ev.Err.Error()
		}
	}
	return nil
}

func (t *turnTransport) Close() error {
	text := t.buf.String()
	if text == "" && t.errMsg != "" {
		text = "错误：" + t.errMsg
	}
	if text == "" || t.msgr == nil {
		return nil
	}
	ctx := context.Background()
	switch t.reply.Kind {
	case ReplyC2C:
		return t.msgr.ReplyC2C(ctx, t.reply.OpenID, t.reply.MsgID, text)
	case ReplyGroup:
		return t.msgr.ReplyGroup(ctx, t.reply.OpenID, t.reply.MsgID, text)
	default:
		return errors.New("unknown qq reply kind")
	}
}
