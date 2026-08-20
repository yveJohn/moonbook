package jobmonitor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type httpRepository struct {
	filters Filters
	page    int
	size    int
}

func (repository *httpRepository) List(_ context.Context, filters Filters, page, size int) ([]Job, int64, error) {
	repository.filters, repository.page, repository.size = filters, page, size
	return []Job{{ID: "9007199254740993", Module: "novel", JobType: "fixture", Status: "failed", LastErrorMessage: "authorization=secret"}}, 1, nil
}

func (*httpRepository) Get(_ context.Context, id string) (Detail, error) {
	if id == "404" {
		return Detail{}, ErrNotFound
	}
	return Detail{Job: Job{ID: id, Module: "novel", JobType: "fixture", Status: "failed", LastErrorMessage: "token=secret"}, Attempts: []Attempt{{ID: "9007199254740994", JobID: id, AttemptNumber: 1, ErrorMessage: "cookie=secret"}}}, nil
}

func TestHTTPListAndDetailContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &httpRepository{}
	router := gin.New()
	RegisterRoutes(router.Group("/"), NewService(repository))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/platform/jobs?module=novel&status=failed&page=2&pageSize=10", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	row := data["list"].([]any)[0].(map[string]any)
	if row["id"] != "9007199254740993" || row["lastErrorMessage"] != "错误信息包含敏感字段，已脱敏" || data["page"] != float64(2) || data["pageSize"] != float64(10) {
		t.Fatalf("unexpected list response: %v", envelope)
	}
	if repository.filters.Module != "novel" || repository.page != 2 || repository.size != 10 {
		t.Fatalf("repository input filters=%+v page=%d size=%d", repository.filters, repository.page, repository.size)
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/platform/jobs/9007199254740993", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"9007199254740993"`) || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("detail status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHTTPRejectsInvalidInputsAndMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/"), NewService(&httpRepository{}))
	for _, path := range []string{
		"/platform/jobs?page=zero", "/platform/jobs?page=0", "/platform/jobs?pageSize=101",
		"/platform/jobs?status=unknown", "/platform/jobs?from=not-a-time", "/platform/jobs/9223372036854775808",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"errorCode":"INVALID_ARGUMENT"`) {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/platform/jobs/404", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"errorCode":"NOT_FOUND"`) {
		t.Fatalf("not found status=%d body=%s", response.Code, response.Body.String())
	}
}
