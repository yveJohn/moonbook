package recharge

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type expiryRepositoryStub struct {
	mu      sync.Mutex
	results []int64
	errors  []error
	calls   int
	batches []int
	windows []time.Duration
	now     []time.Time
	called  chan struct{}
}

func (stub *expiryRepositoryStub) ExpireBatch(_ context.Context, now time.Time, window time.Duration, batch int) (int64, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	index := stub.calls
	stub.calls++
	stub.batches = append(stub.batches, batch)
	stub.windows = append(stub.windows, window)
	stub.now = append(stub.now, now)
	if stub.called != nil {
		select {
		case stub.called <- struct{}{}:
		default:
		}
	}
	var result int64
	var err error
	if index < len(stub.results) {
		result = stub.results[index]
	}
	if index < len(stub.errors) {
		err = stub.errors[index]
	}
	return result, err
}

func (stub *expiryRepositoryStub) snapshot() (int, []int, []time.Duration, []time.Time) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.calls, append([]int(nil), stub.batches...), append([]time.Duration(nil), stub.windows...), append([]time.Time(nil), stub.now...)
}

func TestExpiryWorkerRunOnceUsesBoundedBatchAndClock(t *testing.T) {
	now := time.Date(2026, 8, 16, 1, 2, 3, 0, time.UTC)
	repository := &expiryRepositoryStub{results: []int64{7}}
	worker := &ExpiryWorker{Repo: repository, BatchSize: 25, UnknownReleaseWindow: 17 * time.Minute, Now: func() time.Time { return now }}

	expired, err := worker.RunOnce(context.Background())
	if err != nil || expired != 7 {
		t.Fatalf("expired=%d err=%v", expired, err)
	}
	calls, batches, windows, times := repository.snapshot()
	if calls != 1 || len(batches) != 1 || batches[0] != 25 || windows[0] != 17*time.Minute || !times[0].Equal(now) {
		t.Fatalf("calls=%d batches=%v windows=%v times=%v", calls, batches, windows, times)
	}
}

func TestExpiryWorkerScansImmediatelyAndPeriodically(t *testing.T) {
	repository := &expiryRepositoryStub{called: make(chan struct{}, 4)}
	worker := &ExpiryWorker{Repo: repository, BatchSize: 10, UnknownReleaseWindow: 15 * time.Minute, Interval: 10 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	for index := 0; index < 2; index++ {
		select {
		case <-repository.called:
		case <-time.After(time.Second):
			t.Fatalf("scan %d did not run", index+1)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func TestExpiryWorkerContinuesAfterScanError(t *testing.T) {
	scanErr := errors.New("temporary database failure")
	repository := &expiryRepositoryStub{errors: []error{scanErr, nil}, called: make(chan struct{}, 4)}
	errorSeen := make(chan error, 1)
	worker := &ExpiryWorker{
		Repo: repository, BatchSize: 10, UnknownReleaseWindow: 15 * time.Minute, Interval: 10 * time.Millisecond,
		OnError: func(err error) { errorSeen <- err },
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	select {
	case err := <-errorSeen:
		if !errors.Is(err, scanErr) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first scan error was not reported")
	}
	for index := 0; index < 2; index++ {
		select {
		case <-repository.called:
		case <-time.After(time.Second):
			t.Fatal("worker did not continue after error")
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestExpiryWorkerRejectsInvalidConfiguration(t *testing.T) {
	for _, worker := range []*ExpiryWorker{
		{},
		{Repo: &expiryRepositoryStub{}, BatchSize: 0, UnknownReleaseWindow: time.Minute},
		{Repo: &expiryRepositoryStub{}, BatchSize: 1, UnknownReleaseWindow: 0},
	} {
		if _, err := worker.RunOnce(context.Background()); err == nil {
			t.Fatalf("worker=%+v", worker)
		}
	}
}
