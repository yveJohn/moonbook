package initialize

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapterclean"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/fetchlog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/importtask"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/txtimport"
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
	txtWorker := &txtimport.Worker{
		DB:           db,
		Jobs:         jobs.NewRepository(db),
		Store:        txtimport.MinIOFileStore{Blobs: blobs},
		Writer:       importtask.ChaptersWriter{Service: chapters.NewService(db, objectstore.NewService(db, blobs))},
		WorkerID:     "novel-txt-import-" + global.GVA_CONFIG.App.Node,
		Lease:        10 * time.Minute,
		PollInterval: time.Second,
	}
	cleanWorker := &chapterclean.Worker{
		DB: db, Jobs: jobs.NewRepository(db), AI: aiconfig.NewService(db, os.LookupEnv),
		Objects: objectstore.NewService(db, blobs), WorkerID: "novel-chapter-clean-" + global.GVA_CONFIG.App.Node,
		Lease: 10 * time.Minute, PollInterval: time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	importWorker.Lock()
	importWorker.cancel, importWorker.done = cancel, done
	importWorker.Unlock()
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			if err := worker.Run(ctx); err != nil {
				zap.L().Error("论坛导入任务停止", zap.Error(err))
			}
		}()
		go func() {
			defer wg.Done()
			if err := cleanWorker.Run(ctx); err != nil {
				zap.L().Error("章节清洗任务停止", zap.Error(err))
			}
		}()
		go func() {
			defer wg.Done()
			if err := txtWorker.Run(ctx); err != nil {
				zap.L().Error("TXT导入任务停止", zap.Error(err))
			}
		}()
		wg.Wait()
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
