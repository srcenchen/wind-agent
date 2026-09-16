package message

type EventType string

var (
	EventError   EventType = "error"
	EventReason  EventType = "reason"
	EventDone    EventType = "done"
	EventContent EventType = "content"
)

// Event 事件推送
type Event struct {
	Type EventType
	Text string
	Err  error
}

// Inbound 入口事件
type Inbound struct {
	SessionId string
	Content   string
}
