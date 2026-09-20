package repo

import (
	"context"
	"encoding/json"

	"wind-agent/internal/data/dao"
	"wind-agent/internal/data/model"
	"wind-agent/internal/domain"
)

type SessionRepo struct {
	sessionDao *dao.SessionDao
}

func NewSessionRepo(sessionDao *dao.SessionDao) *SessionRepo {
	return &SessionRepo{sessionDao: sessionDao}
}

var _ domain.SessionRepo = (*SessionRepo)(nil)

func (r *SessionRepo) Create(ctx context.Context, s domain.Session) error {
	return r.sessionDao.Create(ctx, &model.Session{
		SessionID: s.SessionID,
		Provider:  s.Provider,
		Title:     s.Title,
	})
}

func (r *SessionRepo) Delete(ctx context.Context, sessionID string) error {
	return r.sessionDao.Delete(ctx, sessionID)
}

func (r *SessionRepo) Get(ctx context.Context, sessionID string) (domain.Session, bool, error) {
	m, err := r.sessionDao.Get(ctx, sessionID)
	if err != nil || m == nil {
		return domain.Session{}, false, err
	}
	return domain.Session{
		SessionID: m.SessionID,
		Provider:  m.Provider,
		Title:     m.Title,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, true, nil
}

func (r *SessionRepo) List(ctx context.Context) ([]domain.SessionSummary, error) {
	rows, err := r.sessionDao.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SessionSummary, 0, len(rows))
	for _, m := range rows {
		out = append(out, domain.SessionSummary{
			SessionID: m.SessionID,
			Provider:  m.Provider,
			Title:     m.Title,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		})
	}
	return out, nil
}

func (r *SessionRepo) Messages(ctx context.Context, sessionID string) ([]domain.Message, error) {
	rows, err := r.sessionDao.Messages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Message, 0, len(rows))
	for _, m := range rows {
		out = append(out, toDomainMessage(m))
	}
	return out, nil
}

func (r *SessionRepo) Append(ctx context.Context, sessionID string, msgs []domain.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	turn, err := r.sessionDao.NextTurn(ctx, sessionID)
	if err != nil {
		return err
	}
	rows := make([]model.SessionMessage, 0, len(msgs))
	for _, m := range msgs {
		rows = append(rows, toModelMessage(sessionID, turn, m))
	}
	if err := r.sessionDao.AppendMessages(ctx, rows); err != nil {
		return err
	}
	return r.sessionDao.Touch(ctx, sessionID, "")
}

func (r *SessionRepo) Touch(ctx context.Context, sessionID, provider string) error {
	return r.sessionDao.Touch(ctx, sessionID, provider)
}

func toDomainMessage(m model.SessionMessage) domain.Message {
	out := domain.Message{
		Role:       domain.Role(m.Role),
		Content:    m.Content,
		Reason:     m.Reason,
		ToolCallID: m.ToolCallID,
	}
	if m.ToolCalls != "" {
		_ = json.Unmarshal([]byte(m.ToolCalls), &out.ToolCalls)
	}
	return out
}

func toModelMessage(sessionID string, turn int, m domain.Message) model.SessionMessage {
	row := model.SessionMessage{
		SessionID:  sessionID,
		Turn:       turn,
		Role:       string(m.Role),
		Content:    m.Content,
		Reason:     m.Reason,
		ToolCallID: m.ToolCallID,
	}
	if len(m.ToolCalls) > 0 {
		if raw, err := json.Marshal(m.ToolCalls); err == nil {
			row.ToolCalls = string(raw)
		}
	}
	return row
}
