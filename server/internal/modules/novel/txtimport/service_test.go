package txtimport

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

type memoryBlobStore struct {
	key, hash string
	data      []byte
}

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
