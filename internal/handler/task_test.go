package handler

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

func TestGetTaskResultMissingRepoReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/task", nil)

	GetTaskResult(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestCreateTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldLogger := logger.Log
	oldTaskQueue := engine.TaskQuene
	logger.Log = zap.NewNop()
	t.Cleanup(func() {
		logger.Log = oldLogger
		engine.TaskQuene = oldTaskQueue
	})

	t.Run("rejects invalid request body", func(t *testing.T) {
		engine.TaskQuene = make(chan string, 1)
		response := createTaskRequest(`{"target":"github","url":"not-a-url"}`)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
	})

	t.Run("accepts a valid task and enqueues its URL", func(t *testing.T) {
		engine.TaskQuene = make(chan string, 1)
		response := createTaskRequest(`{"target":"github","url":"https://github.com/vuejs/vue"}`)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		if queuedURL := <-engine.TaskQuene; queuedURL != "https://github.com/vuejs/vue" {
			t.Fatalf("queued URL = %q, want %q", queuedURL, "https://github.com/vuejs/vue")
		}
	})

	t.Run("rejects a task when the queue is full", func(t *testing.T) {
		engine.TaskQuene = make(chan string, 1)
		engine.TaskQuene <- "https://github.com/already/queued"
		response := createTaskRequest(`{"target":"github","url":"https://github.com/golang/go"}`)

		if response.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusTooManyRequests)
		}
	})
}

func createTaskRequest(body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/task", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")
	CreateTask(context)
	return recorder
}
