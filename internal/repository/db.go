package repository

import (
	"go-lua-crawler/internal/config"
	"go-lua-crawler/internal/logger"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var DB *gorm.DB

type GithubRepo struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex;not null"`
	Stars       int    `gorm:"default:0"`
	Description string `gorm:"type:text"`
}

func InitDB() {
	cfg:=config.Get()
	dsn:=cfg.MySQL.DSN
	if dsn==""{
		dsn="data.db"
	}
	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Log.Fatal("数据库连接失败", zap.Error(err))
	}
	logger.Log.Info("数据库连接成功", zap.String("type", "sqlite"))
	err = DB.AutoMigrate(&GithubRepo{})
	if err != nil {
		logger.Log.Fatal("自动建表失败", zap.Error(err))
	}
	logger.Log.Info("数据库表结构同步完成")
}
