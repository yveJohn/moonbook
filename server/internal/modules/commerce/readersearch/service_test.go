package readersearch

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type repositoryStub struct {
	upserted []Projection
	deleted  []int64
	page     Page
	err      error
}

func (stub *repositoryStub) Upsert(_ context.Context, projection Projection) error {
	stub.upserted = append(stub.upserted, projection)
	return stub.err
}

func (stub *repositoryStub) Delete(_ context.Context, readerID int64) error {
	stub.deleted = append(stub.deleted, readerID)
	return stub.err
}

func (stub *repositoryStub) Page(_ context.Context, afterReaderID int64, limit int) (Page, error) {
	stub.page.AfterReaderID = afterReaderID
	stub.page.Limit = limit
	return stub.page, stub.err
}

func TestServiceUpsertNormalizesAndDelegates(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository)

	err := service.Upsert(context.Background(), Projection{
		ReaderID: 21,
		Username: "  reader-21  ",
		Nickname: "  Moon Reader  ",
		Status:   " enabled ",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Projection{{ReaderID: 21, Username: "reader-21", Nickname: "Moon Reader", Status: "enabled"}}
	if !reflect.DeepEqual(repository.upserted, want) {
		t.Fatalf("upserted = %+v, want %+v", repository.upserted, want)
	}
}

func TestServiceRejectsInvalidProjection(t *testing.T) {
	tests := []Projection{
		{},
		{ReaderID: 1, Username: "", Status: "enabled"},
		{ReaderID: 1, Username: "reader", Status: "locked"},
		{ReaderID: 1, Username: string(make([]byte, 65)), Status: "enabled"},
		{ReaderID: 1, Username: "reader", Nickname: string(make([]byte, 65)), Status: "enabled"},
	}
	for _, input := range tests {
		repository := &repositoryStub{}
		err := NewService(repository).Upsert(context.Background(), input)
		if !errors.Is(err, ErrInvalidProjection) {
			t.Fatalf("Upsert(%+v) error = %v", input, err)
		}
		if len(repository.upserted) != 0 {
			t.Fatalf("invalid projection reached repository: %+v", repository.upserted)
		}
	}
}

func TestServiceDeleteAndPageValidateArguments(t *testing.T) {
	repository := &repositoryStub{page: Page{Items: []Projection{{ReaderID: 11}}, NextReaderID: 11, Done: false}}
	service := NewService(repository)

	if err := service.Delete(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repository.deleted, []int64{9}) {
		t.Fatalf("deleted = %v", repository.deleted)
	}
	page, err := service.Page(context.Background(), 10, 100)
	if err != nil {
		t.Fatal(err)
	}
	if page.AfterReaderID != 10 || page.Limit != 100 || page.NextReaderID != 11 {
		t.Fatalf("page = %+v", page)
	}

	for _, test := range []struct {
		name  string
		after int64
		limit int
	}{
		{name: "negative cursor", after: -1, limit: 10},
		{name: "zero limit", limit: 0},
		{name: "oversized limit", limit: MaxPageSize + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Page(context.Background(), test.after, test.limit); !errors.Is(err, ErrInvalidPage) {
				t.Fatalf("Page error = %v", err)
			}
		})
	}
	if err := service.Delete(context.Background(), 0); !errors.Is(err, ErrInvalidProjection) {
		t.Fatalf("Delete error = %v", err)
	}
}

func TestServicePreservesRepositoryErrors(t *testing.T) {
	want := errors.New("repository unavailable")
	repository := &repositoryStub{err: want}
	service := NewService(repository)

	if err := service.Upsert(context.Background(), Projection{ReaderID: 1, Username: "reader", Status: "enabled"}); !errors.Is(err, want) {
		t.Fatalf("Upsert error = %v", err)
	}
	if err := service.Delete(context.Background(), 1); !errors.Is(err, want) {
		t.Fatalf("Delete error = %v", err)
	}
	if _, err := service.Page(context.Background(), 0, 10); !errors.Is(err, want) {
		t.Fatalf("Page error = %v", err)
	}
}
