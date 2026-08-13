package initialize

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

func RecoverPlatformJobs() (int64, error) {
	if global.GVA_DB == nil {
		return 0, errors.New("database is not initialized")
	}
	db, err := global.GVA_DB.DB()
	if err != nil {
		return 0, fmt.Errorf("open platform job database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return jobs.NewRepository(db).RecoverExpired(ctx)
}
