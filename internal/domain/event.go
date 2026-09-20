package domain

type EventType string

var (
	EventError      EventType = "error"
	EventReason     EventType = "reason"
	EventDone       EventType = "done"
	EventContent    EventType = "content"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
)

// Event 事件推送
type Event struct {
	Type     EventType
	Text     string
	Err      error
	ToolCall *ToolCall // Type == EventToolCall 时带
}

// Mention 一次 @ 的对象：ID 用于回复时 @ 回去，Name 是昵称（可能为空）。
type Mention struct {
	ID   string
	Name string
}

// Inbound 入口事件
type Inbound struct {
	SessionId string
	Provider  string // 本轮指定的 provider，空则用会话默认
	Content   string
	// Speaker 群聊等共享会话中的发送者 ID，私聊为空。
	Speaker string
	// SpeakerName 发送者昵称，可能为空。
	SpeakerName string
	// Mentions 本轮被 @ 的用户，仅群聊等共享会话有值。
	Mentions []Mention
	// SystemPrompt 传输层附带的额外 system 指令（如群聊的 @ 规则），由传输层自行注入。
	SystemPrompt string
}
