package model

import "gorm.io/gorm"

type Session struct {
	gorm.Model
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
	History   string `json:"history"`
}

type Message struct {
	gorm.Model
}
