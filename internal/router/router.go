package router

import(
	"go-lua-crawler/internal/handler"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	v1:=r.Group("/api/v1")
	{
		v1.POST("/task",handler.CreateTask)

		v1.GET("/task",handler.GetTaskResult)

	}
	return r
}
