package activity

import (
	"context"
	"errors"
	"testing"
	"time"
)

type captureRepository struct {
	readerID int64
	day      time.Time
	at       time.Time
	err      error
}

func (r *captureRepository) Record(_ context.Context, readerID int64, day, at time.Time) error {
	r.readerID, r.day, r.at = readerID, day, at
	return r.err
}
func (*captureRepository) Overview(context.Context, time.Time, *time.Location) (Overview, error) {
	return Overview{}, nil
}

func TestRecordUsesKualaLumpurCalendarDate(t *testing.T) {
	repo := &captureRepository{}
	service := NewService(repo)
	at := time.Date(2035, 1, 1, 16, 0, 1, 0, time.UTC)
	if err := service.Record(context.Background(), 9007199254740993, at); err != nil {
		t.Fatal(err)
	}
	if repo.readerID != 9007199254740993 || repo.day.Format(time.DateOnly) != "2035-01-02" || !repo.at.Equal(at) {
		t.Fatalf("recorded reader=%d day=%s at=%s", repo.readerID, repo.day, repo.at)
	}
}

func TestRecordRejectsInvalidReaderAndReturnsRepositoryFailure(t *testing.T) {
	repo := &captureRepository{err: errors.New("database unavailable")}
	service := NewService(repo)
	if err := service.Record(context.Background(), 0, time.Now()); err == nil {
		t.Fatal("expected invalid reader rejection")
	}
	if err := service.Record(context.Background(), 1, time.Now()); err == nil {
		t.Fatal("expected repository error")
	}
}
