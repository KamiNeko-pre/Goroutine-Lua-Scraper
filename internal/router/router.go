package router

import (
	"github.com/gin-gonic/gin"
	"go-lua-crawler/internal/handler"
)

func SetupRouter() *gin.Engine {
	// Gin 默认路由器已经包含日志和 panic recovery 中间件。
	r := gin.Default()

	// 统一使用版本化 API 前缀，为以后增加 v2 预留兼容空间。
	v1 := r.Group("/api/v1")
	{
		v1.POST("/task", handler.CreateTask)

		v1.GET("/task", handler.GetTaskResult)

	}
	return r
}
