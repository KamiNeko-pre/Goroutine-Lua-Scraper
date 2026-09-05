package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go-lua-crawler/internal/repository"
	taskmodel "go-lua-crawler/internal/task"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateCrawlTaskRequest struct {
	RequestKey string `json:"request_key" binding:"required,max=128"`
	Target     string `json:"target" binding:"required,max=64"`
	URL        string `json:"url" binding:"required,url"`
}

func GetCrawlTaskResult(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid task id"})
		return
	}
	var result repository.CrawlResult
	err = repository.DB.WithContext(c.Request.Context()).First(&result, "task_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"message": "result not available"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "load result failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": id, "data": json.RawMessage(result.Payload)})
}

func CreateCrawlTask(c *gin.Context) {
	var request CreateCrawlTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid request", "error": err.Error()})
		return
	}

	crawlTask := taskmodel.CrawlTask{
		RequestKey: request.RequestKey,
		Target:     request.Target,
		URL:        request.URL,
	}
	if err := repository.CreateQueuedCrawlTaskWithOutbox(c.Request.Context(), &crawlTask); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			existingTask, lookupErr := repository.GetCrawlTaskByRequestKey(c.Request.Context(), request.RequestKey)
			if lookupErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "load idempotent task failed"})
				return
			}
			if existingTask.Target != request.Target || existingTask.URL != request.URL {
				c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "message": "request_key was used for a different task"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": existingTask, "idempotent_replay": true})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "create task failed"})
		return
	}
	repository.RecordTaskEventBestEffort(c.Request.Context(), taskmodel.TaskEvent{
		TaskID:  crawlTask.ID,
		Stage:   taskmodel.EventStageAccepted,
		Level:   taskmodel.EventLevelInfo,
		Message: "任务已受理并写入待投递队列",
	})
	repository.RecordTaskEventBestEffort(c.Request.Context(), taskmodel.TaskEvent{
		TaskID:  crawlTask.ID,
		Stage:   taskmodel.EventStageOutboxCreated,
		Level:   taskmodel.EventLevelInfo,
		Message: "已创建 Outbox，等待 Publisher 投递",
	})

	c.JSON(http.StatusAccepted, gin.H{"code": http.StatusAccepted, "data": crawlTask})
}

func GetCrawlTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid task id"})
		return
	}

	crawlTask, err := repository.GetCrawlTask(c.Request.Context(), id)
	if err != nil {
		if repository.IsCrawlTaskNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "get task failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": crawlTask})
}
