package provider

import (
	"context"
	"database/sql"
	"errors"

	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type Account struct{ db *sql.DB }

var (
	_ readercontract.AccountReader = (*Account)(nil)
	_ readercontract.AccountLocker = (*Account)(nil)
)

func NewAccount(db *sql.DB) *Account { return &Account{db: db} }

func (provider *Account) Account(ctx context.Context, readerID int64) (readercontract.Account, error) {
	if provider == nil || provider.db == nil || readerID <= 0 {
		return readercontract.Account{}, readercontract.ErrAccountNotFound
	}
	return provider.read(ctx, transaction.Executor(ctx, provider.db), readerID, false)
}

func (provider *Account) LockAccount(ctx context.Context, readerID int64) (readercontract.Account, error) {
	if provider == nil || provider.db == nil || readerID <= 0 {
		return readercontract.Account{}, readercontract.ErrAccountNotFound
	}
	executor := transaction.Executor(ctx, provider.db)
	if executor == provider.db {
		return readercontract.Account{}, readercontract.Wrap(readercontract.ErrUnavailable, transaction.ErrNoTransaction)
	}
	return provider.read(ctx, executor, readerID, true)
}

func (provider *Account) read(ctx context.Context, executor transaction.DBTX, readerID int64, lock bool) (readercontract.Account, error) {
	query := `SELECT id,username,nickname,status FROM reader_accounts WHERE id=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var account readercontract.Account
	if err := executor.QueryRowContext(ctx, query, readerID).Scan(&account.ID, &account.Username, &account.Nickname, &account.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return readercontract.Account{}, readercontract.ErrAccountNotFound
		}
		return readercontract.Account{}, readercontract.Wrap(readercontract.ErrUnavailable, err)
	}
	if account.Status == "deleted" {
		return readercontract.Account{}, readercontract.ErrAccountNotFound
	}
	if account.Status != "enabled" {
		return readercontract.Account{}, readercontract.ErrAccountDisabled
	}
	return account, nil
}
