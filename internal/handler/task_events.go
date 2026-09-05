package handler

import (
	"net/http"
	"strconv"

	"go-lua-crawler/internal/repository"

	"github.com/gin-gonic/gin"
)

// ListTaskEvents returns a trace for an existing task. An existing task with
// no event rows receives an empty array so the frontend can render that state.
func ListTaskEvents(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid task id"})
		return
	}
	if repository.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "database is not initialized"})
		return
	}
	if _, err := repository.GetCrawlTask(c.Request.Context(), id); err != nil {
		if repository.IsCrawlTaskNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "get task failed"})
		return
	}
	events, err := repository.ListTaskEvents(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "list task events failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": events})
}
