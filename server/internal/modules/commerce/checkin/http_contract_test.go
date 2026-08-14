package checkin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

const checkinReaderMaxID int64 = 9223372036854775807

type checkinContractRepo struct {
	statusReaderID  int64
	checkinReaderID int64
}

func (r *checkinContractRepo) Status(_ context.Context, readerID int64) (Status, error) {
	r.statusReaderID = readerID
	return Status{TodayRewardCoin: 9007199254740993, RewardRandom: true, CheckinAvailable: true}, nil
}

func (r *checkinContractRepo) Checkin(_ context.Context, readerID int64) (Status, error) {
	r.checkinReaderID = readerID
	return Status{TodayChecked: true, ContinuousDays: 7, TodayRewardCoin: checkinReaderMaxID, RewardText: "获得边界奖励", CheckinAvailable: false, UnavailableReason: "今日已签到"}, nil
}

func TestCheckinHTTPContractKeepsRewardAsStringAndNulls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &checkinContractRepo{}
	handler := &Handler{service: NewService(repo)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: checkinReaderMaxID})
		c.Next()
	})
	router.GET("/reader/me/checkin/status", handler.status)
	router.POST("/reader/me/checkin", handler.checkin)

	tests := []struct {
		name, method, path, want string
	}{
		{"status", http.MethodGet, "/reader/me/checkin/status", `{"code":200,"msg":"查询成功","data":{"todayChecked":false,"continuousDays":0,"todayRewardCoin":"9007199254740993","rewardRandom":true,"rewardText":null,"checkinAvailable":true,"unavailableReason":null}}`},
		{"checkin", http.MethodPost, "/reader/me/checkin", `{"code":200,"msg":"签到成功","data":{"todayChecked":true,"continuousDays":7,"todayRewardCoin":"9223372036854775807","rewardRandom":false,"rewardText":"获得边界奖励","checkinAvailable":false,"unavailableReason":"今日已签到"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(test.method, test.path, nil))
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			var want, got any
			if err := json.Unmarshal([]byte(test.want), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("response mismatch\nwant: %s\n got: %s", test.want, resp.Body.String())
			}
		})
	}
	if repo.statusReaderID != checkinReaderMaxID || repo.checkinReaderID != checkinReaderMaxID {
		t.Fatalf("status reader=%d checkin reader=%d", repo.statusReaderID, repo.checkinReaderID)
	}
}
