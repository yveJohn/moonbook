package initialize

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"go.uber.org/zap"
)

var objectGC struct {
	sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func StartObjectGC() error {
	StopObjectGC()
	if global.GVA_DB == nil {
		return fmt.Errorf("object GC database is not initialized")
	}
	db, err := global.GVA_DB.DB()
	if err != nil {
		return fmt.Errorf("open object GC database: %w", err)
	}
	minio := global.GVA_CONFIG.Minio
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: minio.Endpoint, AccessKey: minio.AccessKeyId, SecretKey: minio.AccessKeySecret, Bucket: minio.BucketName, UseSSL: minio.UseSSL})
	if err != nil {
		return err
	}
	worker, err := objectstore.NewGCWorker(db, objectstore.NewService(db, store), objectstore.GCWorkerConfig{
		WorkerID: "object-gc-" + global.GVA_CONFIG.App.Node, Interval: time.Hour, Lease: 10 * time.Minute,
		GracePeriod: 24 * time.Hour, BatchSize: 500, PollInterval: time.Second,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	objectGC.Lock()
	objectGC.cancel, objectGC.done = cancel, done
	objectGC.Unlock()
	go func() {
		defer close(done)
		if err := worker.Run(ctx); err != nil {
			zap.L().Error("对象回收任务停止", zap.Error(err))
		}
	}()
	return nil
}

func StopObjectGC() {
	objectGC.Lock()
	cancel, done := objectGC.cancel, objectGC.done
	objectGC.cancel, objectGC.done = nil, nil
	objectGC.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			zap.L().Warn("等待对象回收任务停止超时")
		}
	}
}
