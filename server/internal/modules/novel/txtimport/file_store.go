package txtimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

type MinIOFileStore struct{ Blobs objectstore.BlobStore }

func (s MinIOFileStore) Put(ctx context.Context, key string, data []byte, contentType string) (ObjectMeta, error) {
	if s.Blobs == nil {
		return ObjectMeta{}, errors.New("file object store is required")
	}
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, "imports/txt/") || !strings.HasSuffix(key, ".txt") {
		return ObjectMeta{}, errors.New("invalid TXT object key")
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if err := s.Blobs.Put(ctx, key, strings.NewReader(string(data)), int64(len(data)), contentType, hash); err != nil {
		return ObjectMeta{}, err
	}
	stat, err := s.Blobs.Stat(ctx, key)
	if err != nil {
		return ObjectMeta{}, err
	}
	if stat.Key != key || stat.ByteSize != int64(len(data)) || !strings.EqualFold(stat.SHA256, hash) {
		return ObjectMeta{}, errors.New("TXT object verification mismatch")
	}
	return ObjectMeta{Key: key, SHA256: hash, ByteSize: stat.ByteSize, ContentType: stat.ContentType}, nil
}

func (s MinIOFileStore) Get(ctx context.Context, meta ObjectMeta) ([]byte, error) {
	if s.Blobs == nil {
		return nil, errors.New("file object store is required")
	}
	body, err := s.Blobs.Get(ctx, meta.Key)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != meta.ByteSize || sha256Hex(data) != strings.ToLower(meta.SHA256) {
		return nil, errors.New("TXT object verification mismatch")
	}
	return data, nil
}
