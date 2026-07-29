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

// InitCron 根据配置注册周期性回抓任务。未配置 cron 表达式时，
// 定时更新是可选能力，服务仍可继续运行并接受手动创建的任务。
func InitCron() {
	cfg := config.Get()
	spec := cfg.Cron.Spec

	if spec == "" {
		logger.Log.Warn("未配置Cron表达式")
		return
	}

	CronEngine = cron.New()

	// AddFunc 只注册回调；Start 后 cron 会在自己的 goroutine 中按表达式触发它。
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
	// 从数据库读取已知仓库，并将其重新投递到与 HTTP 接口共用的队列。
	// 因此定时任务也会受到同一份 worker 并发上限和队列背压保护。
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
			// 不阻塞 cron 调度线程。队列压力大时丢弃本轮更新，等待下一个周期再尝试。
			logger.Log.Warn("流量高峰 主动丢弃该任务", zap.String("url", targetURL))
		}
	}
	logger.Log.Info("本次定时任务结束", zap.Int("成功投递数量", successCount))
}
