package service

import (
	"context"

	"wind-agent/internal/domain"
	"wind-agent/internal/session"
)

// SessionService 会话用例层：建会话、取会话、跑一轮对话。
type SessionService struct {
	repo domain.SessionRepo
	ag   *session.Agent
}

func NewSessionService(repo domain.SessionRepo, ag *session.Agent) *SessionService {
	return &SessionService{repo: repo, ag: ag}
}

// Chat 由传输层造好 Transport 后传进来，service 只负责驱动对话。
func (s *SessionService) Chat(ctx context.Context, in domain.Inbound, t session.Transport) error {
	return s.ag.RunSessionLoop(ctx, t)
}
