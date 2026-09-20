package qqbot

import (
	"context"
	"errors"
	"io"
	"testing"

	"wind-agent/internal/domain"
	"wind-agent/internal/session"
)

type fakeMessenger struct {
	c2c    []sentMsg
	group  []sentMsg
	errC2C error
}

type sentMsg struct {
	OpenID  string
	MsgID   string
	Content string
}

func (f *fakeMessenger) ReplyC2C(_ context.Context, userOpenID, msgID, content string) error {
	if f.errC2C != nil {
		return f.errC2C
	}
	f.c2c = append(f.c2c, sentMsg{OpenID: userOpenID, MsgID: msgID, Content: content})
	return nil
}

func (f *fakeMessenger) ReplyGroup(_ context.Context, groupOpenID, msgID, content string) error {
	f.group = append(f.group, sentMsg{OpenID: groupOpenID, MsgID: msgID, Content: content})
	return nil
}

func TestQQTransportReceiveOnceThenEOF(t *testing.T) {
	in := domain.Inbound{SessionId: "qq:c2c:u", Content: "hi"}
	tr := newTurnTransport(in, ReplyTarget{Kind: ReplyC2C, OpenID: "u", MsgID: "m"}, &fakeMessenger{})
	var _ session.Transport = tr

	got, err := tr.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hi" {
		t.Fatalf("got %+v", got)
	}
	_, err = tr.Receive(context.Background())
	if !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF, got %v", err)
	}
}

func TestQQTransportSendsAccumulatedContentOnClose(t *testing.T) {
	msgr := &fakeMessenger{}
	in := domain.Inbound{SessionId: "qq:c2c:u", Content: "hi"}
	tr := newTurnTransport(in, ReplyTarget{Kind: ReplyC2C, OpenID: "u", MsgID: "mid"}, msgr)

	ctx := context.Background()
	if err := tr.Emit(ctx, domain.Event{Type: domain.EventReason, Text: "think"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Emit(ctx, domain.Event{Type: domain.EventContent, Text: "你"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Emit(ctx, domain.Event{Type: domain.EventContent, Text: "好"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Emit(ctx, domain.Event{Type: domain.EventDone}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if len(msgr.c2c) != 1 {
		t.Fatalf("sent %d", len(msgr.c2c))
	}
	if msgr.c2c[0].Content != "你好" || msgr.c2c[0].MsgID != "mid" {
		t.Fatalf("%+v", msgr.c2c[0])
	}
}

func TestQQTransportSendsErrorWhenNoContent(t *testing.T) {
	msgr := &fakeMessenger{}
	in := domain.Inbound{SessionId: "qq:c2c:u", Content: "hi"}
	tr := newTurnTransport(in, ReplyTarget{Kind: ReplyC2C, OpenID: "u", MsgID: "mid"}, msgr)
	ctx := context.Background()
	_ = tr.Emit(ctx, domain.Event{Type: domain.EventError, Err: errors.New("boom")})
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if len(msgr.c2c) != 1 || msgr.c2c[0].Content != "错误：boom" {
		t.Fatalf("%+v", msgr.c2c)
	}
}

func TestQQTransportGroupReply(t *testing.T) {
	msgr := &fakeMessenger{}
	in := domain.Inbound{SessionId: "qq:group:g:u", Content: "hi"}
	tr := newTurnTransport(in, ReplyTarget{Kind: ReplyGroup, OpenID: "g", MsgID: "mid"}, msgr)
	ctx := context.Background()
	_ = tr.Emit(ctx, domain.Event{Type: domain.EventContent, Text: "ok"})
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if len(msgr.group) != 1 || msgr.group[0].OpenID != "g" || msgr.group[0].Content != "ok" {
		t.Fatalf("%+v", msgr.group)
	}
}

func TestQQTransportGroupRendersMentions(t *testing.T) {
	msgr := &fakeMessenger{}
	in := domain.Inbound{SessionId: "qq:group:g", Content: "hi", Speaker: "U1"}
	tr := newTurnTransport(in, ReplyTarget{Kind: ReplyGroup, OpenID: "g", MsgID: "mid"}, msgr)
	ctx := context.Background()
	_ = tr.Emit(ctx, domain.Event{Type: domain.EventContent, Text: "你好 @[U2] 和 @[U3]"})
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if len(msgr.group) != 1 || msgr.group[0].Content != "你好 <@U2> 和 <@U3>" {
		t.Fatalf("%+v", msgr.group)
	}
}

func TestQQTransportC2CKeepsTextUnchanged(t *testing.T) {
	msgr := &fakeMessenger{}
	tr := newTurnTransport(domain.Inbound{Content: "hi"}, ReplyTarget{Kind: ReplyC2C, OpenID: "u", MsgID: "mid"}, msgr)
	_ = tr.Emit(context.Background(), domain.Event{Type: domain.EventContent, Text: "@[U2] 你好"})
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if len(msgr.c2c) != 1 || msgr.c2c[0].Content != "@[U2] 你好" {
		t.Fatalf("%+v", msgr.c2c)
	}
}

func TestQQTransportCloseWithoutTextDoesNotSend(t *testing.T) {
	msgr := &fakeMessenger{}
	tr := newTurnTransport(domain.Inbound{Content: "x"}, ReplyTarget{Kind: ReplyC2C, OpenID: "u", MsgID: "m"}, msgr)
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if len(msgr.c2c) != 0 {
		t.Fatalf("unexpected send %+v", msgr.c2c)
	}
}
