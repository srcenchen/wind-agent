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

// Inbound 入口事件
type Inbound struct {
	SessionId string
	Provider  string // 本轮指定的 provider，空则用会话默认
	Content   string
}
