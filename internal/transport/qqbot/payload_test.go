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
				"member_openid": "MEMBER1",
				"username": "小明"
			},
			"content": " 今天天气 ",
			"group_openid": "GROUP1",
			"mentions": [
				{"member_openid": "MEMBER2", "username": "小红"}
			]
		}
	}`)
	msg, err := ParseInbound(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Inbound.SessionId != "qq:group:GROUP1" {
		t.Fatalf("session=%s", msg.Inbound.SessionId)
	}
	if msg.Inbound.Content != "今天天气" {
		t.Fatalf("content=%q", msg.Inbound.Content)
	}
	if msg.Inbound.Speaker != "MEMBER1" || msg.Inbound.SpeakerName != "小明" {
		t.Fatalf("speaker=%q name=%q", msg.Inbound.Speaker, msg.Inbound.SpeakerName)
	}
	want := []domain.Mention{{ID: "MEMBER2", Name: "小红"}}
	if len(msg.Inbound.Mentions) != 1 || msg.Inbound.Mentions[0] != want[0] {
		t.Fatalf("mentions=%v", msg.Inbound.Mentions)
	}
	if msg.Reply.Kind != ReplyGroup || msg.Reply.OpenID != "GROUP1" || msg.Reply.MsgID != "ROBOT1.0_g1" {
		t.Fatalf("reply=%+v", msg.Reply)
	}
}

func TestParseGroupMentionsShareOneSession(t *testing.T) {
	payload := func(member, content string) []byte {
		return []byte(`{"op":0,"t":"GROUP_AT_MESSAGE_CREATE","d":{"id":"m-` + member + `","author":{"member_openid":"` + member + `"},"content":"` + content + `","group_openid":"G1","mentions":[{"member_openid":"MEMBER9"}]}}`)
	}
	a, err := ParseInbound(payload("MEMBER1", "你好 <@!MEMBER9>"))
	if err != nil || a == nil {
		t.Fatalf("a: %v %v", a, err)
	}
	b, err := ParseInbound(payload("MEMBER2", "在吗"))
	if err != nil || b == nil {
		t.Fatalf("b: %v %v", b, err)
	}
	if a.Inbound.SessionId != b.Inbound.SessionId || a.Inbound.SessionId != "qq:group:G1" {
		t.Fatalf("sessions differ: %s vs %s", a.Inbound.SessionId, b.Inbound.SessionId)
	}
	if a.Inbound.Content != "你好 @[MEMBER9]" {
		t.Fatalf("mention not normalized: %q", a.Inbound.Content)
	}
	if a.Inbound.Speaker == b.Inbound.Speaker {
		t.Fatalf("speakers should differ: %q", a.Inbound.Speaker)
	}
}

func TestParseGroupMentionsFromContent(t *testing.T) {
	// content 内联 @，昵称由 mentions 数组补全
	raw := []byte(`{"op":0,"t":"GROUP_AT_MESSAGE_CREATE","d":{"id":"m1","author":{"member_openid":"ME"},"content":"你好 <@!FRIEND1>","group_openid":"G1","mentions":[{"member_openid":"FRIEND1","username":"小红"}]}}`)
	msg, err := ParseInbound(raw)
	if err != nil || msg == nil {
		t.Fatalf("msg=%v err=%v", msg, err)
	}
	if msg.Inbound.Content != "你好 @[FRIEND1]（小红）" {
		t.Fatalf("content=%q", msg.Inbound.Content)
	}
	if len(msg.Inbound.Mentions) != 1 || msg.Inbound.Mentions[0] != (domain.Mention{ID: "FRIEND1", Name: "小红"}) {
		t.Fatalf("mentions=%v", msg.Inbound.Mentions)
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
