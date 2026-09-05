package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseAnalysisQueryRejectsInvalidTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/analysis/snapshots?from=not-time", nil)

	_, _, _, err := parseAnalysisQuery(context)
	if err == nil {
		t.Fatal("expected invalid time error")
	}
}

func TestParseAnalysisQueryClampsOnlyAtRepositoryBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/analysis/snapshots?limit=999", nil)

	_, _, limit, err := parseAnalysisQuery(context)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if limit != 999 {
		t.Fatalf("limit = %d, want handler to preserve the caller value", limit)
	}
}

func TestListTaskEventsRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: "not-a-number"}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/tasks/not-a-number/events", nil)

	ListTaskEvents(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
