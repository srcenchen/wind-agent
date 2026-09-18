package dao

import (
	"gorm.io/gorm"
)

type SessionDao struct {
	db *gorm.DB
}

func NewSessionDao(db *gorm.DB) *SessionDao {
	return &SessionDao{db: db}
}
