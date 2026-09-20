package domain

import (
	"context"
	"time"
)

// Session 主会话
type Session struct {
	SessionID string
	Provider  string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionSummary 会话列表项（不含消息体）
type SessionSummary struct {
	SessionID string
	Provider  string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionRepo 会话与轮次消息的持久化端口。
// Session 与 SessionMessage 是一对多：一个会话包含多轮（turn）消息。
type SessionRepo interface {
	Create(ctx context.Context, s Session) error
	Delete(ctx context.Context, sessionID string) error
	Get(ctx context.Context, sessionID string) (Session, bool, error)
	List(ctx context.Context) ([]SessionSummary, error)
	// Messages 返回会话全部轮次消息，按写入顺序排列（不含 system prompt）。
	Messages(ctx context.Context, sessionID string) ([]Message, error)
	// Append 把一轮对话新增的消息写入会话，自动递增 turn。
	Append(ctx context.Context, sessionID string, msgs []Message) error
	// Touch 刷新会话活跃时间，provider 非空时同时更新默认 provider。
	Touch(ctx context.Context, sessionID, provider string) error
}
