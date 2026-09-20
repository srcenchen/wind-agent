package model

import "gorm.io/gorm"

// Session 主会话，一条记录对应一个持续会话。
type Session struct {
	gorm.Model
	SessionID string           `gorm:"uniqueIndex;size:128;not null" json:"session_id"`
	Provider  string           `gorm:"size:64" json:"provider"`
	Title     string           `gorm:"size:255" json:"title"`
	Messages  []SessionMessage `gorm:"foreignKey:SessionID;references:SessionID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
}

// SessionMessage 会话内的一轮消息，多个消息共享同一个 Turn。
type SessionMessage struct {
	gorm.Model
	SessionID  string `gorm:"index;size:128;not null" json:"session_id"`
	Turn       int    `gorm:"index" json:"turn"`
	Role       string `gorm:"size:32" json:"role"`
	Content    string `json:"content"`
	Reason     string `json:"reason"`
	ToolCalls  string `json:"tool_calls"`
	ToolCallID string `json:"tool_call_id"`
}
