package initialize

import (
	"context"
	"sync"
	"testing"
	"time"
)

type expiryRunnerStub struct {
	started chan struct{}
	stopped chan struct{}
	once    sync.Once
}

func (runner *expiryRunnerStub) Run(ctx context.Context) error {
	close(runner.started)
	<-ctx.Done()
	runner.once.Do(func() { close(runner.stopped) })
	return nil
}

func TestRechargeExpiryLifecycleRestartsAndStopsWorker(t *testing.T) {
	original := buildRechargeExpiryRunner
	defer func() {
		StopRechargeExpiry()
		buildRechargeExpiryRunner = original
	}()
	first := &expiryRunnerStub{started: make(chan struct{}), stopped: make(chan struct{})}
	second := &expiryRunnerStub{started: make(chan struct{}), stopped: make(chan struct{})}
	runners := []rechargeExpiryRunner{first, second}
	buildRechargeExpiryRunner = func() (rechargeExpiryRunner, error) {
		runner := runners[0]
		runners = runners[1:]
		return runner, nil
	}

	if err := StartRechargeExpiry(); err != nil {
		t.Fatal(err)
	}
	waitLifecycleSignal(t, first.started, "first worker start")
	if err := StartRechargeExpiry(); err != nil {
		t.Fatal(err)
	}
	waitLifecycleSignal(t, first.stopped, "first worker stop before restart")
	waitLifecycleSignal(t, second.started, "second worker start")
	StopRechargeExpiry()
	waitLifecycleSignal(t, second.stopped, "second worker stop")
	StopRechargeExpiry()
}

func waitLifecycleSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}
