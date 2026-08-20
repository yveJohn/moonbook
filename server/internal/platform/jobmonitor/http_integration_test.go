//go:build integration

package jobmonitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/gin-gonic/gin"
)

func TestHTTPWithPostgreSQLPreservesIDsAndRedactsAttempts(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id := int64(9007199254740995)
	module := fmt.Sprintf("jobmonitor-http-it-%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_jobs
        (id,module,job_type,idempotency_key,status,attempt_count,max_attempts,last_error_code,last_error_message)
        VALUES($1,$2,'http-fixture',$3,'failed',1,3,'HTTP_FIXTURE','postgres://reader:secret@example.test/moonbook')`, id, module, module); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform_job_attempts WHERE job_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform_jobs WHERE id=$1`, id)
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_job_attempts
        (job_id,attempt_number,worker_id,outcome,error_code,error_message)
        VALUES($1,1,'http-worker','failed','HTTP_FIXTURE','authorization=secret')`, id); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/"), NewService(SQLRepository{DB: db}))

	for _, path := range []string{
		"/platform/jobs?module=" + module + "&page=1&pageSize=20",
		fmt.Sprintf("/platform/jobs/%d", id),
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		body := response.Body.String()
		if response.Code != http.StatusOK || !strings.Contains(body, `"id":"9007199254740995"`) || strings.Contains(body, "secret") || strings.Contains(body, "postgres://") {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, body)
		}
	}
}
