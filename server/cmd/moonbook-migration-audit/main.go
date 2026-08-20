package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/reconcile"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacyaudit"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacymigrate"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() { os.Exit(run()) }

func run() int {
	flag.Parse()
	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: moonbook-migration-audit [migration-name]")
		return 2
	}
	migration := "moonbook-v1"
	if flag.NArg() == 1 && strings.TrimSpace(flag.Arg(0)) != "" {
		migration = flag.Arg(0)
	}
	sourceDSN := strings.TrimSpace(os.Getenv("MOONBOOK_LEGACY_MYSQL_DSN"))
	targetDSN := strings.TrimSpace(os.Getenv("MOONBOOK_DATABASE_DSN"))
	if sourceDSN == "" || targetDSN == "" {
		fmt.Fprintln(os.Stderr, "MOONBOOK_LEGACY_MYSQL_DSN and MOONBOOK_DATABASE_DSN are required")
		return 2
	}
	source, err := sql.Open("mysql", sourceDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open source:", err)
		return 1
	}
	defer source.Close()
	target, err := sql.Open("pgx", targetDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open target:", err)
		return 1
	}
	defer target.Close()
	timeout := 10 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("MOONBOOK_AUDIT_TIMEOUT")); raw != "" {
		if parsed, e := time.ParseDuration(raw); e == nil && parsed >= time.Minute && parsed <= 24*time.Hour {
			timeout = parsed
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := legacymigrate.VerifySourceReadOnly(ctx, source); err != nil {
		fmt.Fprintln(os.Stderr, "source read-only check:", err)
		return 1
	}
	report, err := legacyaudit.Build(ctx, source, target, migration)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build audit:", err)
		return 1
	}
	blobs, err := auditObjectStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "object audit configuration:", err)
		return 2
	}
	report.ObjectIntegrity, err = legacyaudit.AuditObjects(ctx, target, blobs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit objects:", err)
		return 1
	}
	finance, err := reconcile.Full(ctx, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit finance:", err)
		return 1
	}
	report.Finance = financeReport(finance)
	data, err := legacyaudit.Encode(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode audit:", err)
		return 1
	}
	fmt.Println(string(data))
	if report.HasFailures() {
		return 1
	}
	return 0
}

func auditObjectStore() (*objectstore.MinIOStore, error) {
	endpoint := strings.TrimSpace(os.Getenv("MOONBOOK_MINIO_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_USER"))
	secretKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_PASSWORD"))
	bucket := strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	useSSL := false
	if raw := strings.TrimSpace(os.Getenv("MOONBOOK_MINIO_USE_SSL")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("MOONBOOK_MINIO_USE_SSL must be true or false")
		}
		useSSL = value
	}
	return objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket, UseSSL: useSSL})
}

func financeReport(report reconcile.FullReport) legacyaudit.FinanceReport {
	result := legacyaudit.FinanceReport{
		WalletsChecked: report.Wallets.Checked, RechargeOrders: report.RechargeOrders,
		PurchaseOrders: report.PurchaseOrders, Callbacks: report.Callbacks,
		MembershipGrants: report.MembershipGrants, Entitlements: report.Entitlements,
		MismatchCount: len(report.Mismatches),
	}
	for _, mismatch := range report.Mismatches {
		if len(result.Samples) >= 100 {
			break
		}
		digest := sha256.Sum256([]byte(mismatch.Domain + "\x00" + mismatch.Key + "\x00" + mismatch.Field))
		result.Samples = append(result.Samples, legacyaudit.FinanceIssue{Domain: mismatch.Domain, Field: mismatch.Field, Fingerprint: hex.EncodeToString(digest[:])})
	}
	return result
}
