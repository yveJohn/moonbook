package initialize

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type lifecycleWorker struct {
	active    *atomic.Int32
	maxActive *atomic.Int32
	started   chan<- struct{}
	stopped   chan<- struct{}
}

type blockingLifecycleWorker struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (w blockingLifecycleWorker) Run(context.Context) error {
	w.started <- struct{}{}
	<-w.release
	return nil
}

func (w lifecycleWorker) Run(ctx context.Context) error {
	active := w.active.Add(1)
	for {
		maximum := w.maxActive.Load()
		if active <= maximum || w.maxActive.CompareAndSwap(maximum, active) {
			break
		}
	}
	w.started <- struct{}{}
	<-ctx.Done()
	w.active.Add(-1)
	w.stopped <- struct{}{}
	return ctx.Err()
}

func TestContentWorkerLifecycleSerializesConcurrentReplacements(t *testing.T) {
	var lifecycle contentWorkerLifecycle
	lifecycle.timeout = time.Second
	var active, maximum atomic.Int32
	started := make(chan struct{}, 8)
	stopped := make(chan struct{}, 8)
	build := func() ([]namedContentWorker, error) {
		return []namedContentWorker{{name: "test", worker: lifecycleWorker{active: &active, maxActive: &maximum, started: started, stopped: stopped}}}, nil
	}

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := lifecycle.replace(build); err != nil {
				t.Errorf("replace: %v", err)
			}
		}()
	}
	wg.Wait()
	lifecycle.stop()

	if maximum.Load() != 1 || active.Load() != 0 {
		t.Fatalf("maximum active=%d active=%d", maximum.Load(), active.Load())
	}
	if len(started) != 4 || len(stopped) != 4 {
		t.Fatalf("started=%d stopped=%d", len(started), len(stopped))
	}
}

func TestContentWorkerLifecycleStopThenStartCreatesOneNewGeneration(t *testing.T) {
	var lifecycle contentWorkerLifecycle
	lifecycle.timeout = time.Second
	var active, maximum atomic.Int32
	started := make(chan struct{}, 2)
	stopped := make(chan struct{}, 2)
	build := func() ([]namedContentWorker, error) {
		return []namedContentWorker{{name: "test", worker: lifecycleWorker{active: &active, maxActive: &maximum, started: started, stopped: stopped}}}, nil
	}

	if err := lifecycle.replace(build); err != nil {
		t.Fatal(err)
	}
	<-started
	lifecycle.stop()
	if err := lifecycle.replace(build); err != nil {
		t.Fatal(err)
	}
	<-started
	lifecycle.stop()

	if maximum.Load() != 1 || active.Load() != 0 || len(stopped) != 2 {
		t.Fatalf("maximum active=%d active=%d stopped=%d", maximum.Load(), active.Load(), len(stopped))
	}
}

func TestContentWorkerLifecycleRejectsReplacementWhenPreviousSetDoesNotStop(t *testing.T) {
	var lifecycle contentWorkerLifecycle
	lifecycle.timeout = 20 * time.Millisecond
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	if err := lifecycle.replace(func() ([]namedContentWorker, error) {
		return []namedContentWorker{{name: "blocking", worker: blockingLifecycleWorker{started: started, release: release}}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := lifecycle.replace(func() ([]namedContentWorker, error) { return nil, nil }); err == nil {
		t.Fatal("replacement started before the previous worker set stopped")
	}
	close(release)
	lifecycle.stop()
}
