package logger

import (
	"log"

	"go.uber.org/zap"
)

var Log *zap.Logger

// Init 创建开发环境日志器并赋值给全局变量。其他模块通过 logger.Log 输出结构化日志。
func Init(){
  var err error
  Log,err=zap.NewDevelopment()
  if err!=nil{
	log.Fatalf("初始化日志失败: %v",err)
  }
zap.ReplaceGlobals(Log)
}
