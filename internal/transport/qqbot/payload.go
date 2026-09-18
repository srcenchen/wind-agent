package qqbot

import (
	"encoding/json"
	"fmt"
	"strings"

	"wind-agent/internal/domain"
)

const (
	opDispatch    = 0
	opHeartbeat   = 1
	opIdentify    = 2
	opReconnect   = 7
	opInvalidSess = 9
	opHello       = 10
	opHeartbeatOK = 11
	opCallbackACK = 12
	opValidation  = 13

	eventC2C      = "C2C_MESSAGE_CREATE"
	eventGroupAT  = "GROUP_AT_MESSAGE_CREATE"
	eventGroupMsg = "GROUP_MESSAGE_CREATE"
	eventReady    = "READY"

	// IntentGroupAndC2C = GROUP_AND_C2C_EVENT (1 << 25)
	IntentGroupAndC2C = 1 << 25
)

type ReplyKind int

const (
	ReplyC2C ReplyKind = iota + 1
	ReplyGroup
)

type ReplyTarget struct {
	Kind   ReplyKind
	OpenID string
	MsgID  string
}

type InboundMessage struct {
	Inbound domain.Inbound
	Reply   ReplyTarget
}

type ValidationRequest struct {
	PlainToken string `json:"plain_token"`
	EventTs    string `json:"event_ts"`
}

type envelope struct {
	ID string          `json:"id"`
	Op int             `json:"op"`
	S  *int            `json:"s"`
	T  string          `json:"t"`
	D  json.RawMessage `json:"d"`
}

type messageData struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	GroupOpenID string `json:"group_openid"`
	Author      struct {
		ID           string `json:"id"`
		UserOpenID   string `json:"user_openid"`
		MemberOpenID string `json:"member_openid"`
	} `json:"author"`
}

func ParseInbound(raw []byte) (*InboundMessage, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("qq payload: %w", err)
	}
	if env.Op != opDispatch {
		return nil, nil
	}
	switch env.T {
	case eventC2C, eventGroupAT, eventGroupMsg:
	default:
		return nil, nil
	}
	var d messageData
	if err := json.Unmarshal(env.D, &d); err != nil {
		return nil, fmt.Errorf("qq event data: %w", err)
	}
	content := strings.TrimSpace(d.Content)
	if content == "" {
		return nil, nil
	}
	msg := &InboundMessage{
		Inbound: domain.Inbound{Content: content},
		Reply:   ReplyTarget{MsgID: d.ID},
	}
	switch env.T {
	case eventC2C:
		openid := firstNonEmpty(d.Author.UserOpenID, d.Author.ID)
		if openid == "" || d.ID == "" {
			return nil, nil
		}
		msg.Inbound.SessionId = "qq:c2c:" + openid
		msg.Reply.Kind = ReplyC2C
		msg.Reply.OpenID = openid
	case eventGroupAT, eventGroupMsg:
		member := firstNonEmpty(d.Author.MemberOpenID, d.Author.ID)
		if d.GroupOpenID == "" || member == "" || d.ID == "" {
			return nil, nil
		}
		msg.Inbound.SessionId = "qq:group:" + d.GroupOpenID + ":" + member
		msg.Reply.Kind = ReplyGroup
		msg.Reply.OpenID = d.GroupOpenID
	}
	return msg, nil
}

func ParseHello(raw []byte) (int, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, err
	}
	if env.Op != opHello {
		return 0, fmt.Errorf("not a hello payload: op=%d", env.Op)
	}
	var d struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	if err := json.Unmarshal(env.D, &d); err != nil {
		return 0, err
	}
	if d.HeartbeatInterval <= 0 {
		return 0, fmt.Errorf("invalid heartbeat_interval")
	}
	return d.HeartbeatInterval, nil
}

func ParseReadySessionID(raw []byte) (string, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	if env.Op != opDispatch || env.T != eventReady {
		return "", fmt.Errorf("not a ready event")
	}
	var d struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(env.D, &d); err != nil {
		return "", err
	}
	if d.SessionID == "" {
		return "", fmt.Errorf("empty session_id")
	}
	return d.SessionID, nil
}

func ParseValidation(raw []byte) (ValidationRequest, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ValidationRequest{}, err
	}
	if env.Op != opValidation {
		return ValidationRequest{}, fmt.Errorf("not a validation payload")
	}
	var v ValidationRequest
	if err := json.Unmarshal(env.D, &v); err != nil {
		return ValidationRequest{}, err
	}
	return v, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
