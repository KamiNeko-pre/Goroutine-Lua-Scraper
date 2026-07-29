package main

import (
	"fmt"
	"go-lua-crawler/internal/config"
	"go-lua-crawler/internal/engine"
	"go-lua-crawler/internal/scheduler"

	//"go-lua-crawler/internal/engine"
	"go-lua-crawler/internal/logger"
	"go-lua-crawler/internal/repository"
	"go-lua-crawler/internal/router"

	//"sync"
	"go.uber.org/zap"
)

func main() {
	// 启动顺序很重要：后续组件都会读取配置或使用日志、数据库等全局依赖。
	// 因此先加载配置，再依次初始化基础设施和业务组件。
	// 加载 configs/config.yaml，并启动配置文件监听。
	config.InitConfig()
	// 任务队列是 HTTP 请求和后台 worker 之间的缓冲层。
	// 队列满时，接口会立即返回 429，而不是让请求无限期阻塞。
	engine.TaskQuene = make(chan string, config.Get().Engine.TaskQueueSize)
	// 初始化全局 zap 日志。defer 保证进程退出前尽量刷出缓冲日志。
	logger.Init()
	defer logger.Log.Sync()
	startPprofServer(config.Get().App.PprofPort)
	// 建立数据库连接并同步 GithubRepo 的表结构。
	if err := repository.InitDB(); err != nil {
		logger.Log.Fatal("数据库初始化失败", zap.Error(err))
	}
	// Lua 规则以缓存形式保存在内存，并监听脚本文件变化以支持热更新。
	logger.Log.Info("====启动Lua热更新===")
	engine.InitLuaEngine(config.Get().App.LuaPath)
	// 注册定时任务，将数据库中的历史仓库重新投递到任务队列。
	logger.Log.Info("======启动自动调度引擎=====")
	scheduler.InitCron()

	// 启动固定数量的 worker。每个 worker 串行消费一个任务，
	// 多个 worker 共同构成受 WorkerCount 控制的并发上限。
	logger.Log.Info("=====后台工人团队开始孵化======")
	for i := 1; i <= config.Get().Engine.WorkerCount; i++ {
		go func(workerID int) {
			// range 会一直等待新任务；只有通道被关闭后循环才会退出。
			for task := range engine.TaskQuene {
				logger.Log.Info("工人抢到订单,开始干活", zap.Int("工人编号", workerID), zap.String("目标", task))
				err := engine.RunLuaScript(config.Get().App.LuaPath, task)
				if err != nil {
					logger.Log.Error("抓取失败", zap.Int("工人", workerID), zap.String("原因", err.Error()))
				} else {
					logger.Log.Info("抓取并入库成功", zap.Int("工人", workerID), zap.String("目标", task))
				}
			}
		}(i)
	}

	logger.Log.Info("========工人团队准备完成", zap.Int("工人数", config.Get().Engine.WorkerCount))
	// 组装 HTTP 路由后，Gin 开始监听并阻塞主 goroutine。
	r := router.SetupRouter()

	addr := fmt.Sprintf(":%d", config.Get().App.Port)
	logger.Log.Info("===========爬虫系统启动成功========", zap.String("监听地址", addr))
	if err := r.Run(addr); err != nil {
		logger.Log.Fatal("服务器启动失败", zap.Error(err))
	}

}
