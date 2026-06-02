package logger

import (
	"log"

	"go.uber.org/zap"
)

var Log *zap.Logger

func Init(){
  var err error
  Log,err=zap.NewDevelopment()
  if err!=nil{
	log.Fatalf("初始化日志失败: %v",err)
  }
zap.ReplaceGlobals(Log)
}