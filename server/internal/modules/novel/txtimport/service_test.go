package txtimport

import (
	"context"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

type memoryBlobStore struct {
	key, hash string
	data      []byte
}

type previewRepository struct{ task Task }

type replacingRepository struct {
	id       int64
	filename string
	meta     ObjectMeta
}

func (r previewRepository) List(context.Context, string, string, int, int) ([]Task, int64, error) {
	return nil, 0, nil
}
func (r previewRepository) Get(context.Context, int64) (Task, error)          { return r.task, nil }
func (r previewRepository) Create(context.Context, CreateInput) (Task, error) { return Task{}, nil }
func (r previewRepository) ReplaceFile(context.Context, int64, string, ObjectMeta) (Task, error) {
	return Task{}, nil
}
func (r previewRepository) Retry(context.Context, int64) (Task, error) { return Task{}, nil }
func (r previewRepository) Cancel(context.Context, int64) error        { return nil }

func (r *replacingRepository) List(context.Context, string, string, int, int) ([]Task, int64, error) {
	return nil, 0, nil
}
func (r *replacingRepository) Get(context.Context, int64) (Task, error) {
	return Task{}, nil
}
func (r *replacingRepository) Create(context.Context, CreateInput) (Task, error) {
	return Task{}, nil
}
func (r *replacingRepository) ReplaceFile(_ context.Context, id int64, filename string, meta ObjectMeta) (Task, error) {
	r.id, r.filename, r.meta = id, filename, meta
	return Task{ID: strconv.FormatInt(id, 10), OriginalFilename: filename, Status: "pending"}, nil
}
func (r *replacingRepository) Retry(context.Context, int64) (Task, error) {
	return Task{}, nil
}
func (r *replacingRepository) Cancel(context.Context, int64) error { return nil }

func (m *memoryBlobStore) Put(_ context.Context, key string, body io.Reader, size int64, _ string, hash string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return io.ErrUnexpectedEOF
	}
	m.key, m.hash, m.data = key, hash, append([]byte(nil), data...)
	return nil
}
func (m *memoryBlobStore) Stat(context.Context, string) (objectstore.BlobStat, error) {
	return objectstore.BlobStat{Key: m.key, SHA256: m.hash, ByteSize: int64(len(m.data)), ContentType: "text/plain; charset=utf-8"}, nil
}
func (m *memoryBlobStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(m.data))), nil
}
func (m *memoryBlobStore) Remove(context.Context, string) error { return nil }

func TestValidateTXTFile(t *testing.T) {
	if err := validateFile("book.TXT", []byte("正文")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "book.pdf", "../book.txt"} {
		if err := validateFile(name, []byte("x")); err == nil {
			t.Fatalf("%q should be rejected", name)
		}
	}
	if err := validateFile("book.txt", nil); err == nil {
		t.Fatal("empty file should be rejected")
	}
}

func TestMinIOFileStoreVerifiesRoundTrip(t *testing.T) {
	backend := &memoryBlobStore{}
	store := MinIOFileStore{Blobs: backend}
	data := []byte("第1章\n正文")
	meta, err := store.Put(context.Background(), "imports/txt/example.txt", data, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if meta.ByteSize != int64(len(data)) || meta.SHA256 == "" {
		t.Fatalf("meta=%+v", meta)
	}
	got, err := store.Get(context.Background(), meta)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got=%q", got)
	}
	meta.SHA256 = strings.Repeat("0", 64)
	if _, err := store.Get(context.Background(), meta); err == nil {
		t.Fatal("hash mismatch should fail")
	}
}

func TestPreviewLimitsChaptersAndContent(t *testing.T) {
	data := []byte("第1章\n" + strings.Repeat("长", maxPreviewRunes+5) + "\nChapter 2\n第二部分正文")
	backend := &memoryBlobStore{}
	store := MinIOFileStore{Blobs: backend}
	meta, err := store.Put(context.Background(), "imports/txt/preview.txt", data, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(previewRepository{task: Task{ID: "9", OriginalFilename: "preview.txt", ObjectKey: meta.Key, ObjectSHA256: meta.SHA256, ObjectByteSize: strconv.FormatInt(meta.ByteSize, 10)}})
	preview, err := service.Preview(context.Background(), store, 9)
	if err != nil {
		t.Fatal(err)
	}
	if preview.TotalChapterCount != 2 || len(preview.Chapters) != 2 || !preview.Chapters[0].Truncated || len([]rune(preview.Chapters[0].Content)) != maxPreviewRunes {
		t.Fatalf("preview=%+v", preview)
	}
}

func TestReplaceFileUploadsVerifiedObjectBeforeRepositoryUpdate(t *testing.T) {
	backend := &memoryBlobStore{}
	store := MinIOFileStore{Blobs: backend}
	repo := &replacingRepository{}
	service := NewService(repo)
	item, err := service.ReplaceFile(context.Background(), store, 27, "fixed.txt", []byte("第1章\n修复正文"))
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "27" || item.Status != "pending" || repo.id != 27 || repo.filename != "fixed.txt" {
		t.Fatalf("item=%+v repo=%+v", item, repo)
	}
	if repo.meta.Key == "" || repo.meta.SHA256 == "" || repo.meta.ByteSize == 0 || backend.key != repo.meta.Key {
		t.Fatalf("meta=%+v backend=%+v", repo.meta, backend)
	}
}
