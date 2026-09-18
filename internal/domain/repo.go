package domain

import (
	"time"
)

// SessionSummary 会话列表项（不含消息体）
type SessionSummary struct {
	ID        string
	Provider  string
	UpdatedAt time.Time
}

type SessionRepo interface {
}
