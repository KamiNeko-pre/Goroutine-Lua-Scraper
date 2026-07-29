package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go-lua-crawler/internal/engine"
	"go-lua-crawler/internal/logger"
	"go.uber.org/zap"
)

func TestTaskRouteRejectsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldLogger := logger.Log
	oldTaskQueue := engine.TaskQuene
	logger.Log = zap.NewNop()
	engine.TaskQuene = make(chan string, 1)
	t.Cleanup(func() {
		logger.Log = oldLogger
		engine.TaskQuene = oldTaskQueue
	})

	router := SetupRouter()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/task",
		bytes.NewBufferString(`{"target":"github","url":"invalid-url"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
