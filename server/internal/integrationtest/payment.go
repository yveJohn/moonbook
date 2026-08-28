//go:build integration

package integrationtest

import (
	"context"
	"database/sql"
)

// EnablePaymentChannel prepares the shared integration channel for state-machine
// tests that inject their own gateway and therefore never decrypt these values.
func EnablePaymentChannel(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `UPDATE reader_payment_channels SET
        enabled=true,
        merchant_pid_ciphertext='integration-test-placeholder',
        secret_ciphertext='integration-test-placeholder',
        epusdt_base_url='http://127.0.0.1',
        reader_base_url='http://127.0.0.1',
        create_url='http://127.0.0.1/create',
        notify_url='http://127.0.0.1/notify',
        redirect_url='http://127.0.0.1/redirect',
        health_url='http://127.0.0.1/health',
        sync_url='http://127.0.0.1/sync'
        WHERE provider='epusdt' AND archived_at IS NULL`)
	return err
}
