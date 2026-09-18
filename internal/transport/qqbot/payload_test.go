package qqbot

import (
	"testing"

	"wind-agent/internal/domain"
)

func TestParseC2CMessage(t *testing.T) {
	raw := []byte(`{
		"id": "evt-1",
		"op": 0,
		"s": 1,
		"t": "C2C_MESSAGE_CREATE",
		"d": {
			"id": "ROBOT1.0_msg1",
			"author": {
				"id": "UOPENID",
				"user_openid": "UOPENID"
			},
			"content": "你好"
		}
	}`)
	msg, err := ParseInbound(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil {
		t.Fatal("expected inbound")
	}
	if msg.Inbound.SessionId != "qq:c2c:UOPENID" {
		t.Fatalf("session=%s", msg.Inbound.SessionId)
	}
	if msg.Inbound.Content != "你好" {
		t.Fatalf("content=%q", msg.Inbound.Content)
	}
	if msg.Reply.Kind != ReplyC2C || msg.Reply.OpenID != "UOPENID" || msg.Reply.MsgID != "ROBOT1.0_msg1" {
		t.Fatalf("reply=%+v", msg.Reply)
	}
}

func TestParseGroupATMessage(t *testing.T) {
	raw := []byte(`{
		"op": 0,
		"t": "GROUP_AT_MESSAGE_CREATE",
		"d": {
			"id": "ROBOT1.0_g1",
			"author": {
				"id": "MEMBER1",
				"member_openid": "MEMBER1"
			},
			"content": " 今天天气 ",
			"group_openid": "GROUP1"
		}
	}`)
	msg, err := ParseInbound(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Inbound.SessionId != "qq:group:GROUP1:MEMBER1" {
		t.Fatalf("session=%s", msg.Inbound.SessionId)
	}
	if msg.Inbound.Content != "今天天气" {
		t.Fatalf("content=%q", msg.Inbound.Content)
	}
	if msg.Reply.Kind != ReplyGroup || msg.Reply.OpenID != "GROUP1" || msg.Reply.MsgID != "ROBOT1.0_g1" {
		t.Fatalf("reply=%+v", msg.Reply)
	}
}

func TestParseSkipsUnknownAndEmpty(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"op":1,"d":45000}`),
		[]byte(`{"op":0,"t":"READY","d":{}}`),
		[]byte(`{"op":0,"t":"C2C_MESSAGE_CREATE","d":{"id":"x","author":{"id":"u"},"content":"  "}}`),
	}
	for i, raw := range cases {
		msg, err := ParseInbound(raw)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if msg != nil {
			t.Fatalf("case %d: expected skip, got %+v", i, msg)
		}
	}
}

func TestParseValidation(t *testing.T) {
	raw := []byte(`{"op":13,"d":{"plain_token":"tok","event_ts":"123"}}`)
	v, err := ParseValidation(raw)
	if err != nil {
		t.Fatal(err)
	}
	if v.PlainToken != "tok" || v.EventTs != "123" {
		t.Fatalf("%+v", v)
	}
}

func TestInboundProviderEmpty(t *testing.T) {
	raw := []byte(`{
		"op":0,"t":"C2C_MESSAGE_CREATE",
		"d":{"id":"m","author":{"user_openid":"u"},"content":"hi"}
	}`)
	msg, err := ParseInbound(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Inbound.Provider != "" {
		t.Fatalf("provider should be empty, got %q", msg.Inbound.Provider)
	}
	_ = domain.Inbound{}
}
