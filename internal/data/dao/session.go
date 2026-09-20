package dao

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"wind-agent/internal/data/model"
)

type SessionDao struct {
	db *gorm.DB
}

func NewSessionDao(db *gorm.DB) *SessionDao {
	return &SessionDao{db: db}
}

func (d *SessionDao) Create(ctx context.Context, s *model.Session) error {
	return d.db.WithContext(ctx).Create(s).Error
}

func (d *SessionDao) Get(ctx context.Context, sessionID string) (*model.Session, error) {
	var s model.Session
	err := d.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *SessionDao) Delete(ctx context.Context, sessionID string) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&model.SessionMessage{}).Error; err != nil {
			return err
		}
		return tx.Where("session_id = ?", sessionID).Delete(&model.Session{}).Error
	})
}

func (d *SessionDao) List(ctx context.Context) ([]model.Session, error) {
	var out []model.Session
	err := d.db.WithContext(ctx).Order("updated_at DESC").Find(&out).Error
	return out, err
}

func (d *SessionDao) Messages(ctx context.Context, sessionID string) ([]model.SessionMessage, error) {
	var out []model.SessionMessage
	err := d.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("id ASC").
		Find(&out).Error
	return out, err
}

func (d *SessionDao) NextTurn(ctx context.Context, sessionID string) (int, error) {
	var maxTurn int
	err := d.db.WithContext(ctx).Model(&model.SessionMessage{}).
		Where("session_id = ?", sessionID).
		Select("COALESCE(MAX(turn), 0)").
		Scan(&maxTurn).Error
	if err != nil {
		return 0, err
	}
	return maxTurn + 1, nil
}

func (d *SessionDao) AppendMessages(ctx context.Context, msgs []model.SessionMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).Create(&msgs).Error
}

func (d *SessionDao) Touch(ctx context.Context, sessionID, provider string) error {
	updates := map[string]any{"updated_at": time.Now()}
	if provider != "" {
		updates["provider"] = provider
	}
	return d.db.WithContext(ctx).Model(&model.Session{}).
		Where("session_id = ?", sessionID).
		Updates(updates).Error
}
