package repo

import (
	"wind-agent/internal/data/dao"
)

type SessionRepo struct {
	sessionDao *dao.SessionDao
}

func NewSessionRepo(sessionDao *dao.SessionDao) *SessionRepo {
	return &SessionRepo{sessionDao: sessionDao}
}
