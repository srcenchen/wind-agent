package data

import (
	"fmt"
	"wind-agent/internal/config"
	"wind-agent/internal/data/dao"
	"wind-agent/internal/data/model"
	"wind-agent/internal/data/repo"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Data 是数据层容器：持有 DB 与各个仓储，后续其他仓储也挂这里。
type Data struct {
	DB      *gorm.DB
	Session *repo.SessionRepo
}

func NewData(appCfg config.App) (*Data, error) {
	db, err := gormBuild(appCfg.Database) // 构建持续化数据库
	if err != nil {
		return nil, err
	}
	return &Data{
		DB:      db,
		Session: repo.NewSessionRepo(dao.NewSessionDao(db)),
	}, nil
}

func gormBuild(dbCfg config.Database) (*gorm.DB, error) {
	driver := dbCfg.Driver
	connection := dbCfg.Connection
	var db *gorm.DB
	var err error
	switch driver {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(connection), &gorm.Config{})
	default:
		err = fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		return nil, err
	}
	// AutoMigrate
	err = db.AutoMigrate(&model.Session{}, &model.SessionMessage{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
