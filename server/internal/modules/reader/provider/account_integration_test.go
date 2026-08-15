//go:build integration

package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestAccountProviderReadsAndLocksLiveReaderState(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	enabledID, disabledID, deletedID := base, base+1, base+2
	prefix := fmt.Sprintf("reader-account-provider-%d", base)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id IN ($1,$2,$3)`, enabledID, disabledID, deletedID)
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'启用读者','x','enabled'),($3,$4,'禁用读者','x','disabled'),($5,$6,'删除读者','x','deleted')`, enabledID, prefix+"-enabled", disabledID, prefix+"-disabled", deletedID, prefix+"-deleted"); err != nil {
		t.Fatal(err)
	}

	provider := NewAccount(db)
	account, err := provider.Account(ctx, enabledID)
	if err != nil || account.ID != enabledID || account.Username != prefix+"-enabled" || account.Nickname != "启用读者" || account.Status != "enabled" {
		t.Fatalf("account=%+v err=%v", account, err)
	}
	if _, err := provider.Account(ctx, disabledID); !errors.Is(err, readercontract.ErrAccountDisabled) {
		t.Fatalf("disabled account err=%v", err)
	}
	if _, err := provider.Account(ctx, deletedID); !errors.Is(err, readercontract.ErrAccountNotFound) {
		t.Fatalf("deleted account err=%v", err)
	}
	if _, err := provider.Account(ctx, base+999); !errors.Is(err, readercontract.ErrAccountNotFound) {
		t.Fatalf("missing account err=%v", err)
	}
	if _, err := provider.LockAccount(ctx, enabledID); !errors.Is(err, readercontract.ErrUnavailable) || !errors.Is(readercontract.Cause(err), transaction.ErrNoTransaction) {
		t.Fatalf("lock without transaction err=%v", err)
	}
	if err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		locked, err := provider.LockAccount(txCtx, enabledID)
		if err != nil || locked.ID != enabledID {
			t.Fatalf("locked=%+v err=%v", locked, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
