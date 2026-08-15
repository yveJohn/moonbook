//go:build integration

package accountsync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	commerceprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/provider"
)

func TestProjectionConsistencyRepairWithPostgres(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var baseID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+10 FROM reader_accounts`).Scan(&baseID); err != nil {
		t.Fatal(err)
	}
	readerIDs := []int64{baseID, baseID + 1}
	extraID := baseID + 2
	for index, readerID := range readerIDs {
		if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,$3,'fixture',$4)`, readerID, fmt.Sprintf("account-sync-%d", readerID), fmt.Sprintf("Reader %d", index), []string{"enabled", "disabled"}[index]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,'stale-account','Reader 1','disabled'),($2,'extra-account','Extra','enabled')`, readerIDs[1], extraID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id BETWEEN $1 AND $2`, baseID, extraID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id = ANY($1)`, readerIDs)
	})

	provider := commerceprovider.NewReaderSearch(db)
	service := NewService(SQLAccountPager{DB: db}, provider, provider, 1)
	report, err := service.Check(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Missing != 1 || report.Mismatched != 1 || report.Extra != 1 {
		t.Fatalf("audit report=%+v", report)
	}
	repaired, err := service.Check(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Repaired != 3 {
		t.Fatalf("repair report=%+v", repaired)
	}
	clean, err := service.Check(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Missing != 0 || clean.Mismatched != 0 || clean.Extra != 0 {
		t.Fatalf("projection did not converge: %+v", clean)
	}
}
