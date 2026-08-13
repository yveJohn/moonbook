package managementresponse

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestManagementSuccessEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	OK(context, Page{List: []string{}, Total: 0, Page: 1, PageSize: 10}, "获取成功")
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["code"] != float64(0) || response["msg"] != "获取成功" {
		t.Fatalf("response=%v", response)
	}
	data, ok := response["data"].(map[string]any)
	if !ok || data["page"] != float64(1) || data["pageSize"] != float64(10) || data["total"] != float64(0) {
		t.Fatalf("data=%v", response["data"])
	}

	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	Message(context, "删除成功")
	if recorder.Body.String() != `{"code":0,"data":{},"msg":"删除成功"}` {
		t.Fatalf("message response=%s", recorder.Body.String())
	}
}
