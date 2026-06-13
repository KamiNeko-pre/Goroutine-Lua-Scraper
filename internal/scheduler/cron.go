package scheduler

import (
	"fmt"
	"go-lua-crawler/internal/config"
	"go-lua-crawler/internal/engine"
	"go-lua-crawler/internal/logger"
	"go-lua-crawler/internal/repository"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

var CronEngine *cron.Cron

func InitCron() {
	cfg := config.Get()
	spec := cfg.Cron.Spec

	if spec == "" {
		logger.Log.Warn("未配置Cron表达式")
		return
	}

	CronEngine = cron.New()

	_, err := CronEngine.AddFunc(spec, func() {
		logger.Log.Info("定时更新触发")
		dispatchTasks()
	})
	if err != nil {
		logger.Log.Fatal("定时任务失败", zap.Error(err))
	}
	CronEngine.Start()
	logger.Log.Info("定时任务启动", zap.String("规则", spec))

}

func dispatchTasks() {
	var repos []repository.GithubRepo
	result := repository.DB.Find(&repos)
	if result.Error != nil {
		logger.Log.Error("读取数据库url失败", zap.Error(result.Error))
		return
	}
	if len(repos) == 0 {
		logger.Log.Info("数据库为空，无需自动更新")
		return
	}
	successCount := 0
	for _, repo := range repos {
		targetURL := fmt.Sprintf("https://github.com/%s", repo.Name)
		select {
		case engine.TaskQuene <- targetURL:
			successCount++
		default:
			logger.Log.Warn("流量高峰 主动丢弃该任务", zap.String("url", targetURL))
		}
	}
	logger.Log.Info("本次定时任务结束", zap.Int("成功投递数量", successCount))
}
