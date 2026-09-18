package domain

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall 模型发起的一次工具调用
type ToolCall struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Args string `json:"args"`
}

// Message 一条对话消息
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	Reason     string     `json:"reason"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant 发起
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool 结果对应的调用
}
