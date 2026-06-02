package handler

import (
	//"encoding"
	"go-lua-crawler/internal/engine"
	"go-lua-crawler/internal/logger"
	"go-lua-crawler/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

//定义抓取结构体
type TaskRequest struct {
	Target string `json:"target" binding:"required"`
	URL    string `json:"url" binding:"required,url"`
}
//创建抓取任务函数
func CreateTask(c *gin.Context) {
	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Error("收到非法请求", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误, 请检查是否缺少字段或URL格式不对",
			"err":  err.Error(),
		})
		return
	}
	logger.Log.Info("收到合法抓取任务", zap.String("target", req.Target), zap.String("url", req.URL))
	//非阻塞等待
	select{
	case engine.TaskQuene<-req.URL:
		logger.Log.Info("任务提交滑道成功")
		c.JSON(http.StatusOK,gin.H{
			"code": 200,
			"msg": "任务受理成功,后台开始抓取",
			"data": req,
		})
	default:
		logger.Log.Info("系统繁忙,订单滑道已满,拒绝接单",zap.String("url",req.URL))
		c.JSON(http.StatusTooManyRequests,gin.H{
			"code": 429,
			"msg": "系统当前极度繁忙,请稍后再试",
		})
	}

}

func GetTaskResult(c *gin.Context) {

repoName:= c.Query("repo")
if repoName==""{
	c.JSON(http.StatusBadRequest,gin.H{
		"code": 400,
		"error": "请提供 repo 参数, 例如/api/v1/task?repo=vuejs/vue",
	})
}
logger.Log.Info("读通道收到查询请求",zap.String("repo",repoName))

var repoData repository.GithubRepo

result:=repository.DB.Where("name=?",repoName).First(&repoData)

if result.Error==nil{
	c.JSON(http.StatusOK,gin.H{
		"code": 200,
		"status": "success",
		"source": "database",
		"data": repoData,
	})
	return
}

c.JSON(http.StatusOK,gin.H{
	"code": 200,
	"status": "pending",
	"msg": "暂无该仓库的抓取结果。如果是新任务，可能后台工人加急处理中，请稍后刷新再看",
})



}




