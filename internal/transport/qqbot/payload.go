package qqbot

import (
	"encoding/json"
	"fmt"
	"regexp"
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
		Username     string `json:"username"`
		UserOpenID   string `json:"user_openid"`
		MemberOpenID string `json:"member_openid"`
	} `json:"author"`
	Mentions []mentionData `json:"mentions"`
}

// mentionData 消息中被 @ 的对象：群聊用 member_openid，单聊用 user_openid。
type mentionData struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	UserOpenID   string `json:"user_openid"`
	MemberOpenID string `json:"member_openid"`
}

func (m mentionData) mentionID() string {
	return firstNonEmpty(m.MemberOpenID, m.UserOpenID, m.ID)
}

// mentionTokenRE 匹配 content 内联的 @ 占位符，如 <@!openid> / <@openid>。
var mentionTokenRE = regexp.MustCompile(`<@!?([^>]+)>`)

// markdownSystemPrompt QQ 单聊/群聊支持 markdown，允许模型用它排版。
const markdownSystemPrompt = "可以使用 QQ 支持的 markdown 语法排版回复（如 # 标题、**加粗**、- 列表、[链接](url)、> 引用）。"

// groupSystemPrompt 群聊专属的 system 指令，由本传输层注入，说明发送者标记与 @ 用法。
const groupSystemPrompt = "这是群聊会话，可能有多个用户发言。\n" +
	"消息格式：[发送者id]（昵称）说：内容，方括号里是发送者的用户 id。\n" +
	"被 @ 的人显示为 @[用户id]（昵称），本轮被 @ 的名单会列在消息末尾的 [本轮@: ...]。\n" +
	"当你需要 @ 某位群成员时，必须在回复正文里原样写出 @[该用户id]（只写方括号里的 id，例如 @[ED09E6BA949216652AD95E430C491BD9] 你好呀），系统会自动转成真正的 @；只能 @ 消息里出现过的用户 id。"

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
		msg.Inbound.SystemPrompt = markdownSystemPrompt
		msg.Reply.Kind = ReplyC2C
		msg.Reply.OpenID = openid
	case eventGroupAT, eventGroupMsg:
		member := firstNonEmpty(d.Author.MemberOpenID, d.Author.ID)
		if d.GroupOpenID == "" || member == "" || d.ID == "" {
			return nil, nil
		}
		// 群聊共用一个 session，发送者与被 @ 的人通过 Speaker/Mentions 传给模型。
		// content 已由 QQ 去掉 @机器人的前缀，@ 其他用户只出现在 mentions 列表里。
		msg.Inbound.SessionId = "qq:group:" + d.GroupOpenID
		arrayMentions := toMentions(d.Mentions)
		msg.Inbound.Content = normalizeMentions(content, mentionNames(arrayMentions))
		msg.Inbound.Speaker = member
		msg.Inbound.SpeakerName = d.Author.Username
		msg.Inbound.Mentions = mergeMentions(arrayMentions, contentMentions(content))
		msg.Inbound.SystemPrompt = markdownSystemPrompt + "\n" + groupSystemPrompt
		msg.Reply.Kind = ReplyGroup
		msg.Reply.OpenID = d.GroupOpenID
	}
	return msg, nil
}

// toMentions 把 payload 的 mentions 转成 domain.Mention。
func toMentions(mentions []mentionData) []domain.Mention {
	out := make([]domain.Mention, 0, len(mentions))
	for _, m := range mentions {
		id := m.mentionID()
		if id == "" {
			continue
		}
		out = append(out, domain.Mention{ID: id, Name: m.Username})
	}
	return out
}

// mentionNames 建立 id -> 昵称 映射，用于把内联 @ 占位符渲染成昵称。
func mentionNames(mentions []domain.Mention) map[string]string {
	names := make(map[string]string, len(mentions))
	for _, m := range mentions {
		if m.Name != "" {
			names[m.ID] = m.Name
		}
	}
	return names
}

// normalizeMentions 把 content 内联的 @ 占位符替换成 @[id]（昵称）。
func normalizeMentions(content string, names map[string]string) string {
	return mentionTokenRE.ReplaceAllStringFunc(content, func(s string) string {
		m := mentionTokenRE.FindStringSubmatch(s)
		if len(m) < 2 {
			return s
		}
		return "@" + mentionLabel(m[1], names[m[1]])
	})
}

// mentionLabel 渲染 @ 标记：@[id] 或 @[id]（昵称）。
func mentionLabel(id, name string) string {
	if name == "" {
		return "[" + id + "]"
	}
	return "[" + id + "]（" + name + "）"
}

// contentMentions 取出 content 内联 @ 的 id（不含昵称）。
func contentMentions(content string) []domain.Mention {
	matches := mentionTokenRE.FindAllStringSubmatch(content, -1)
	out := make([]domain.Mention, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 && m[1] != "" {
			out = append(out, domain.Mention{ID: m[1]})
		}
	}
	return out
}

// mergeMentions 按出现顺序合并并去重，后出现的昵称可补全前面的空昵称。
func mergeMentions(groups ...[]domain.Mention) []domain.Mention {
	out := make([]domain.Mention, 0)
	index := make(map[string]int)
	for _, g := range groups {
		for _, m := range g {
			if m.ID == "" {
				continue
			}
			if i, ok := index[m.ID]; ok {
				if out[i].Name == "" && m.Name != "" {
					out[i].Name = m.Name
				}
				continue
			}
			index[m.ID] = len(out)
			out = append(out, m)
		}
	}
	return out
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
