package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

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

// CreateSession 新建一个 web 会话。
func (s *SessionService) CreateSession(ctx context.Context, provider, title string) (domain.Session, error) {
	if title == "" {
		title = "新对话"
	}
	sess := domain.Session{
		SessionID: "web:" + uuid.NewString(),
		Provider:  provider,
		Title:     title,
	}
	if err := s.repo.Create(ctx, sess); err != nil {
		return domain.Session{}, err
	}
	created, _, err := s.repo.Get(ctx, sess.SessionID)
	if err != nil {
		return domain.Session{}, err
	}
	return created, nil
}

// DeleteSession 删除会话及其全部消息。
func (s *SessionService) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return errors.New("session_id is required")
	}
	return s.repo.Delete(ctx, sessionID)
}

// ListSessions 返回会话列表，按活跃时间倒序。
func (s *SessionService) ListSessions(ctx context.Context) ([]domain.SessionSummary, error) {
	return s.repo.List(ctx)
}

// GetSession 返回会话及其历史消息。
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (domain.Session, []domain.Message, bool, error) {
	sess, ok, err := s.repo.Get(ctx, sessionID)
	if err != nil || !ok {
		return domain.Session{}, nil, ok, err
	}
	msgs, err := s.repo.Messages(ctx, sessionID)
	if err != nil {
		return domain.Session{}, nil, true, err
	}
	return sess, msgs, true, nil
}
