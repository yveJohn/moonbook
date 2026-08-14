//go:build integration

// Package integrationtest contains opt-in tests against the local Moonbook
// dependencies. It never creates or modifies production resources.
package integrationtest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	DSN, RedisAddr, RedisPassword                 string
	MinIOEndpoint, MinIOAccessKey, MinIOSecretKey string
}

func LoadConfig(t *testing.T) Config {
	t.Helper()
	c := Config{
		DSN:            os.Getenv("MOONBOOK_READER_TEST_DSN"),
		RedisAddr:      os.Getenv("MOONBOOK_READER_TEST_REDIS_ADDR"),
		RedisPassword:  os.Getenv("MOONBOOK_READER_TEST_REDIS_PASSWORD"),
		MinIOEndpoint:  os.Getenv("MOONBOOK_READER_TEST_MINIO_ENDPOINT"),
		MinIOAccessKey: os.Getenv("MOONBOOK_READER_TEST_MINIO_ACCESS_KEY"),
		MinIOSecretKey: os.Getenv("MOONBOOK_READER_TEST_MINIO_SECRET_KEY"),
	}
	return c
}

func RequireDB(t *testing.T) (*sql.DB, Config) {
	t.Helper()
	c := LoadConfig(t)
	if c.DSN == "" {
		t.Skip("MOONBOOK_READER_TEST_DSN 未配置")
	}
	db, err := sql.Open("pgx", c.DSN)
	if err != nil {
		t.Fatalf("打开 PostgreSQL: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("连接 PostgreSQL: %v", err)
	}
	for _, table := range []string{"reader_accounts", "reader_sessions", "reader_invite_codes", "reader_invite_relations", "reader_bookshelf_entries", "reader_book_likes", "reader_reading_history", "reader_reading_preferences", "reader_feedback", "reader_daily_activity", "reader_wallets", "reader_wallet_ledgers", "reader_recharge_products", "reader_recharge_settings", "reader_payment_channels", "reader_recharge_orders", "reader_payment_callback_logs", "commerce_products", "commerce_membership_grants", "commerce_entitlements", "novel_books", "novel_chapters", "novel_objects"} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil || !exists {
			db.Close()
			t.Fatalf("迁移表 %s 不存在，请先执行项目迁移", table)
		}
	}
	t.Cleanup(func() { db.Close() })
	return db, c
}

func RequireRedis(t *testing.T, c Config) *redis.Client {
	t.Helper()
	if c.RedisAddr == "" {
		t.Skip("MOONBOOK_READER_TEST_REDIS_ADDR 未配置")
	}
	r := redis.NewClient(&redis.Options{Addr: c.RedisAddr, Password: c.RedisPassword, DB: 0})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.Ping(ctx).Err(); err != nil {
		r.Close()
		t.Fatalf("连接 Redis: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

type MinIO struct {
	Store  *objectstore.MinIOStore
	Client *minio.Client
	Bucket string
}

func RequireMinIO(t *testing.T, db *sql.DB, c Config) MinIO {
	t.Helper()
	if c.MinIOEndpoint == "" || c.MinIOAccessKey == "" || c.MinIOSecretKey == "" {
		t.Skip("MOONBOOK_READER_TEST_MINIO_ENDPOINT/ACCESS_KEY/SECRET_KEY 未配置")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	endpoint := strings.TrimPrefix(strings.TrimPrefix(c.MinIOEndpoint, "https://"), "http://")
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(c.MinIOAccessKey, c.MinIOSecretKey, "")})
	if err != nil {
		t.Fatalf("创建 MinIO 客户端: %v", err)
	}
	bucket := fmt.Sprintf("moonbook-reader-it-%d", time.Now().UnixNano())
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("创建测试 bucket: %v", err)
	}
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: c.MinIOEndpoint, AccessKey: c.MinIOAccessKey, SecretKey: c.MinIOSecretKey, Bucket: bucket})
	if err != nil {
		t.Fatalf("创建 MinIO store: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		for obj := range client.ListObjects(cleanupCtx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if obj.Err == nil {
				_ = client.RemoveObject(cleanupCtx, bucket, obj.Key, minio.RemoveObjectOptions{})
			}
		}
		_ = client.RemoveBucket(cleanupCtx, bucket)
	})
	return MinIO{Store: store, Client: client, Bucket: bucket}
}

func Prefix() string { return fmt.Sprintf("it-%d", time.Now().UnixNano()) }
