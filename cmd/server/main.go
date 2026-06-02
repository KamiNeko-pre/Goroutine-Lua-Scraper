package main

import (
	"fmt"
	"go-lua-crawler/internal/config"
	"go-lua-crawler/internal/engine"

	//"go-lua-crawler/internal/engine"
	"go-lua-crawler/internal/logger"
	"go-lua-crawler/internal/repository"
	"go-lua-crawler/internal/router"

	//"sync"

	"go.uber.org/zap"
)

func main(){
	//加载配置config.yaml文件
	config.InitConfig()
	//初始化zap日志
	logger.Init()
	defer logger.Log.Sync()
	//初始化数据库
	repository.InitDB()
	//初始化热更新
	logger.Log.Info("====启动Lua热更新===")
	engine.InitLuaEngine(config.Get().App.LuaPath)

    //启动协程
	logger.Log.Info("=====后台工人团队开始孵化======")
	for i:=1;i<=3;i++{
		go func(workerID int){
			for task:=range engine.TaskQuene{
				logger.Log.Info("工人抢到订单,开始干活",zap.Int("工人编号",workerID),zap.String("目标",task))
				err:=engine.RunLuaScript(config.Get().App.LuaPath,task)
				if err!=nil{
					logger.Log.Error("抓取失败",zap.Int("工人",workerID),zap.String("原因",err.Error()))
				}else{
					logger.Log.Info("抓取并入库成功",zap.Int("工人",workerID),zap.String("目标",task))
				}
			}
		}(i)
	}

	logger.Log.Info("========3台工人已就位,死盯订单滑道=====")
	//定义路由
	r:=router.SetupRouter()

	addr:=fmt.Sprintf(":%d",config.Get().App.Port)
	logger.Log.Info("===========爬虫系统启动成功========",zap.String("监听地址",addr))
	if err:=r.Run(addr);err!=nil{
		logger.Log.Fatal("服务器启动失败",zap.Error(err))
	}

}

 