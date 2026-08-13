package objectstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type BlobStore interface {
	Put(context.Context, string, io.Reader, int64, string, string) error
	Stat(context.Context, string) (BlobStat, error)
	Get(context.Context, string) (io.ReadCloser, error)
	Remove(context.Context, string) error
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type MinIOStore struct {
	client *minio.Client
	bucket string
}

func NewMinIOStore(config MinIOConfig) (*MinIOStore, error) {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(config.Endpoint), "https://"), "http://")
	if endpoint == "" || config.AccessKey == "" || config.SecretKey == "" || strings.TrimSpace(config.Bucket) == "" {
		return nil, fmt.Errorf("MinIO endpoint, credentials and bucket are required")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}
	return &MinIOStore{client: client, bucket: config.Bucket}, nil
}

func (store *MinIOStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType, sha256 string) error {
	_, err := store.client.PutObject(ctx, store.bucket, key, body, size, minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: map[string]string{"sha256": sha256},
	})
	if err != nil {
		return fmt.Errorf("put MinIO object: %w", err)
	}
	return nil
}

func (store *MinIOStore) Stat(ctx context.Context, key string) (BlobStat, error) {
	info, err := store.client.StatObject(ctx, store.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return BlobStat{}, fmt.Errorf("stat MinIO object: %w", err)
	}
	return BlobStat{
		Key:         info.Key,
		SHA256:      info.Metadata.Get("X-Amz-Meta-Sha256"),
		ByteSize:    info.Size,
		ContentType: info.ContentType,
	}, nil
}

func (store *MinIOStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := store.client.GetObject(ctx, store.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get MinIO object: %w", err)
	}
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, fmt.Errorf("open MinIO object: %w", err)
	}
	return object, nil
}

func (store *MinIOStore) Remove(ctx context.Context, key string) error {
	err := store.client.RemoveObject(ctx, store.bucket, key, minio.RemoveObjectOptions{})
	if err == nil {
		return nil
	}
	response := minio.ToErrorResponse(err)
	if response.StatusCode == http.StatusNotFound || response.Code == "NoSuchKey" {
		return nil
	}
	return fmt.Errorf("remove MinIO object: %w", err)
}
