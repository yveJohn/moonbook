package initialize

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/fetchlog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/importtask"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
	"go.uber.org/zap"
)

var importWorker struct {
	sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func StartImportWorker() error {
	StopImportWorker()
	if global.GVA_DB == nil {
		return fmt.Errorf("import worker database is not initialized")
	}
	db, err := global.GVA_DB.DB()
	if err != nil {
		return fmt.Errorf("open import worker database: %w", err)
	}
	minio := global.GVA_CONFIG.Minio
	blobs, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: minio.Endpoint, AccessKey: minio.AccessKeyId, SecretKey: minio.AccessKeySecret, Bucket: minio.BucketName, UseSSL: minio.UseSSL})
	if err != nil {
		return fmt.Errorf("initialize import worker object store: %w", err)
	}
	executor, err := importtask.NewHTTPExecutor(30*time.Second, 4<<20)
	if err != nil {
		return err
	}
	worker := &importtask.Worker{
		DB:           db,
		Jobs:         jobs.NewRepository(db),
		Logs:         fetchlog.SQLRepository{DB: db},
		Executor:     executor,
		Writer:       importtask.ChaptersWriter{Service: chapters.NewService(db, objectstore.NewService(db, blobs))},
		WorkerID:     "novel-import-" + global.GVA_CONFIG.App.Node,
		Lease:        10 * time.Minute,
		PollInterval: time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	importWorker.Lock()
	importWorker.cancel, importWorker.done = cancel, done
	importWorker.Unlock()
	go func() {
		defer close(done)
		if err := worker.Run(ctx); err != nil {
			zap.L().Error("小说导入任务停止", zap.Error(err))
		}
	}()
	return nil
}

func StopImportWorker() {
	importWorker.Lock()
	cancel, done := importWorker.cancel, importWorker.done
	importWorker.cancel, importWorker.done = nil, nil
	importWorker.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			zap.L().Warn("等待小说导入任务停止超时")
		}
	}
}
