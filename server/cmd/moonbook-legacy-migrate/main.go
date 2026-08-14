package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacymigrate"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() { os.Exit(run()) }

func run() int {
	flag.Parse()
	if flag.NArg() != 1 || (flag.Arg(0) != "all" && flag.Arg(0) != "preflight" && flag.Arg(0) != "novel-metadata" && flag.Arg(0) != "novel-books" && flag.Arg(0) != "novel-chapters" && flag.Arg(0) != "novel-reader-seo" && flag.Arg(0) != "reader-identity" && flag.Arg(0) != "reader-commerce" && flag.Arg(0) != "reader-finance") {
		fmt.Fprintln(os.Stderr, "usage: moonbook-legacy-migrate [all|preflight|novel-metadata|novel-books|novel-chapters|novel-reader-seo|reader-identity|reader-commerce|reader-finance]")
		return 2
	}
	sourceDSN := strings.TrimSpace(os.Getenv("MOONBOOK_LEGACY_MYSQL_DSN"))
	targetDSN := strings.TrimSpace(os.Getenv("MOONBOOK_DATABASE_DSN"))
	if sourceDSN == "" || targetDSN == "" {
		fmt.Fprintln(os.Stderr, "MOONBOOK_LEGACY_MYSQL_DSN and MOONBOOK_DATABASE_DSN are required")
		return 2
	}
	batchSize := 1000
	if raw := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_BATCH_SIZE")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			fmt.Fprintln(os.Stderr, "MOONBOOK_MIGRATION_BATCH_SIZE must be an integer")
			return 2
		}
		batchSize = value
	}
	source, err := sql.Open("mysql", sourceDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open legacy MySQL:", err)
		return 1
	}
	defer source.Close()
	target, err := sql.Open("pgx", targetDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open PostgreSQL:", err)
		return 1
	}
	defer target.Close()
	runner, err := legacymigrate.NewRunner(source, target, "moonbook-v1", batchSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	timeout, err := migrationTimeout(os.Getenv("MOONBOOK_MIGRATION_TIMEOUT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := flag.Arg(0)
	stages := []legacymigrate.Stage{legacymigrate.PreflightStage{}}
	if command == "all" {
		objects, downloader, err := coverDependencies(target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		stages = append(stages,
			legacymigrate.NovelCategoryDictionaryStage{},
			legacymigrate.LegacyBookCategoryStage{},
			legacymigrate.NovelBookAuthorStage{},
			legacymigrate.LegacyAuthorTableStage{Table: "book_author"},
			legacymigrate.LegacyAuthorTableStage{Table: "author"},
			legacymigrate.NovelBooksStage{},
			legacymigrate.NovelBookSubCategoriesStage{},
			legacymigrate.NovelBookCoversStage{Objects: objects, Downloader: downloader},
			legacymigrate.NovelChaptersStage{Objects: objects},
			legacymigrate.NovelReaderSEOStage{},
			legacymigrate.ReaderIdentityStage{},
			legacymigrate.ReaderCommerceStage{},
			legacymigrate.ReaderFinanceStage{},
		)
	}
	if command == "reader-identity" || command == "reader-commerce" || command == "reader-finance" {
		stages = append(stages, legacymigrate.ReaderIdentityStage{})
		if command == "reader-commerce" || command == "reader-finance" {
			stages = append(stages, legacymigrate.ReaderCommerceStage{})
		}
		if command == "reader-finance" {
			stages = append(stages, legacymigrate.ReaderFinanceStage{})
		}
	}
	if command == "novel-reader-seo" {
		stages = append(stages, legacymigrate.NovelReaderSEOStage{})
	}
	if command == "novel-metadata" || command == "novel-books" || command == "novel-chapters" {
		stages = append(stages,
			legacymigrate.NovelCategoryDictionaryStage{},
			legacymigrate.LegacyBookCategoryStage{},
			legacymigrate.NovelBookAuthorStage{},
			legacymigrate.LegacyAuthorTableStage{Table: "book_author"},
			legacymigrate.LegacyAuthorTableStage{Table: "author"},
		)
	}
	if command == "novel-books" || command == "novel-chapters" {
		objects, downloader, err := coverDependencies(target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		stages = append(stages, legacymigrate.NovelBooksStage{}, legacymigrate.NovelBookSubCategoriesStage{}, legacymigrate.NovelBookCoversStage{Objects: objects, Downloader: downloader})
		if command == "novel-chapters" {
			stages = append(stages, legacymigrate.NovelChaptersStage{Objects: objects})
		}
	}
	if err := runner.Run(ctx, stages...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("migration=moonbook-v1 command=%s status=complete\n", command)
	return 0
}

func migrationTimeout(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 12 * time.Hour, nil
	}
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || value < time.Minute || value > 24*time.Hour {
		return 0, fmt.Errorf("MOONBOOK_MIGRATION_TIMEOUT must be between 1m and 24h")
	}
	return value, nil
}

func coverDependencies(target *sql.DB) (*objectstore.Service, *legacymigrate.CoverDownloader, error) {
	endpoint := strings.TrimSpace(os.Getenv("MOONBOOK_MINIO_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_USER"))
	secretKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_PASSWORD"))
	bucket := strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, nil, fmt.Errorf("object migration requires MOONBOOK_MINIO_ENDPOINT, MINIO_ROOT_USER, MINIO_ROOT_PASSWORD, and MINIO_BUCKET")
	}
	useSSL := false
	if raw := strings.TrimSpace(os.Getenv("MOONBOOK_MINIO_USE_SSL")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("MOONBOOK_MINIO_USE_SSL must be true or false")
		}
		useSSL = value
	}
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket, UseSSL: useSSL})
	if err != nil {
		return nil, nil, err
	}
	timeout, err := legacymigrate.ParseCoverDownloadTimeout(os.Getenv("MOONBOOK_LEGACY_COVER_TIMEOUT_SECONDS"))
	if err != nil {
		return nil, nil, err
	}
	downloader, err := legacymigrate.NewCoverDownloader(timeout, legacymigrate.ParseAllowedCoverHosts(os.Getenv("MOONBOOK_LEGACY_COVER_ALLOWED_HOSTS")))
	if err != nil {
		return nil, nil, err
	}
	return objectstore.NewService(target, store), downloader, nil
}
