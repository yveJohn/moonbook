package initialize

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/bookprofile"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/candidate"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapterclean"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chaptersummary"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/crawlsource"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/fetchlog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/importtask"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/txtimport"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
	"go.uber.org/zap"
)

type contentWorker interface {
	Run(context.Context) error
}

type namedContentWorker struct {
	name   string
	worker contentWorker
}

type contentWorkerLifecycle struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	timeout time.Duration
}

var importWorker contentWorkerLifecycle

func (l *contentWorkerLifecycle) replace(build func() ([]namedContentWorker, error)) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.stopLocked() {
		return errors.New("previous content workers did not stop before timeout")
	}

	workers, err := build()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	l.cancel, l.done = cancel, done
	go runContentWorkers(ctx, done, workers)
	return nil
}

func (l *contentWorkerLifecycle) stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopLocked()
}

func (l *contentWorkerLifecycle) stopLocked() bool {
	cancel, done := l.cancel, l.done
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return true
	}
	timeout := l.timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		l.cancel, l.done = nil, nil
		return true
	case <-timer.C:
		zap.L().Warn("等待内容任务停止超时")
		return false
	}
}

func runContentWorkers(ctx context.Context, done chan<- struct{}, workers []namedContentWorker) {
	defer close(done)
	var wg sync.WaitGroup
	wg.Add(len(workers))
	for _, item := range workers {
		item := item
		go func() {
			defer wg.Done()
			if err := item.worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				zap.L().Error("内容任务停止", zap.String("worker", item.name), zap.Error(err))
			}
		}()
	}
	wg.Wait()
}

func StartImportWorker() error {
	return importWorker.replace(buildContentWorkers)
}

func buildContentWorkers() ([]namedContentWorker, error) {
	if global.GVA_DB == nil {
		return nil, fmt.Errorf("import worker database is not initialized")
	}
	db, err := global.GVA_DB.DB()
	if err != nil {
		return nil, fmt.Errorf("open import worker database: %w", err)
	}
	minio := global.GVA_CONFIG.Minio
	blobs, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: minio.Endpoint, AccessKey: minio.AccessKeyId, SecretKey: minio.AccessKeySecret, Bucket: minio.BucketName, UseSSL: minio.UseSSL})
	if err != nil {
		return nil, fmt.Errorf("initialize import worker object store: %w", err)
	}
	executor, err := importtask.NewHTTPExecutor(30*time.Second, 4<<20)
	if err != nil {
		return nil, err
	}
	forumSecrets := crawlsource.EnvSecretResolver{Lookup: os.LookupEnv}
	worker := &importtask.Worker{
		DB:           db,
		Jobs:         jobs.NewRepository(db),
		Logs:         fetchlog.SQLRepository{DB: db},
		Executor:     executor,
		Writer:       importtask.ChaptersWriter{Service: chapters.NewService(db, objectstore.NewService(db, blobs))},
		Secrets:      forumSecrets,
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
	summaryWorker := &chaptersummary.Worker{
		DB: db, Jobs: jobs.NewRepository(db), AI: aiconfig.NewService(db, os.LookupEnv),
		Objects: objectstore.NewService(db, blobs), WorkerID: "novel-chapter-summary-" + global.GVA_CONFIG.App.Node,
		Lease: 10 * time.Minute, PollInterval: time.Second, AutoScan: 5 * time.Minute,
	}
	profileService := bookprofile.NewService(db, objectstore.NewService(db, blobs))
	profileWorker := &bookprofile.Worker{
		DB: db, Jobs: jobs.NewRepository(db), AI: aiconfig.NewService(db, os.LookupEnv), Service: profileService,
		WorkerID: "novel-book-profile-" + global.GVA_CONFIG.App.Node, Lease: 10 * time.Minute,
		PollInterval: time.Second, AutoScan: 5 * time.Minute,
	}
	discoveryWorker := &candidate.DiscoveryWorker{
		DB: db, WorkerID: "novel-forum-discovery-" + global.GVA_CONFIG.App.Node,
		PollInterval: time.Minute, Client: &http.Client{Timeout: 30 * time.Second},
		Secrets: forumSecrets,
	}
	return []namedContentWorker{
		{name: "forum-import", worker: worker},
		{name: "forum-discovery", worker: discoveryWorker},
		{name: "chapter-clean", worker: cleanWorker},
		{name: "chapter-summary", worker: summaryWorker},
		{name: "book-profile", worker: profileWorker},
		{name: "txt-import", worker: txtWorker},
	}, nil
}

func StopImportWorker() {
	importWorker.stop()
}
