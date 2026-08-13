package initialize

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/health"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/migrate"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func runtimeHealthChecker() *health.Checker {
	return health.NewChecker(2*time.Second, map[string]health.Probe{
		"minio":      minioHealthProbe(),
		"migrations": migrationHealthProbe,
		"postgres":   postgresHealthProbe,
		"redis":      redisHealthProbe,
	})
}

func postgresHealthProbe(ctx context.Context) error {
	if global.GVA_DB == nil {
		return errors.New("database is not initialized")
	}
	db, err := global.GVA_DB.DB()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

func migrationHealthProbe(ctx context.Context) error {
	if global.GVA_DB == nil {
		return errors.New("database is not initialized")
	}
	db, err := global.GVA_DB.DB()
	if err != nil {
		return err
	}
	status, err := migrate.CurrentStatus(ctx, db)
	if err != nil {
		return err
	}
	if status.Pending || status.Current != status.Target {
		return errors.New("database migrations are pending")
	}
	return nil
}

func redisHealthProbe(ctx context.Context) error {
	if global.GVA_REDIS == nil {
		return errors.New("redis is not initialized")
	}
	return global.GVA_REDIS.Ping(ctx).Err()
}

func minioHealthProbe() health.Probe {
	cfg := global.GVA_CONFIG.Minio
	return func(ctx context.Context) error {
		endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")
		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.AccessKeyId, cfg.AccessKeySecret, ""),
			Secure: cfg.UseSSL,
		})
		if err != nil {
			return err
		}
		exists, err := client.BucketExists(ctx, cfg.BucketName)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("bucket does not exist")
		}
		return nil
	}
}
