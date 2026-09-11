//go:build integration

package adminmembership

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestAdminMembershipGrantIsIdempotentAndAdditive(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300006000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, readerID, "membership-admin-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, "membership-admin-"+suffix); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_membership_grants WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	account := readerprovider.NewAccount(db)
	service := NewService(SQLRepository{DB: db}, transaction.New(db), account)
	first, err := service.Grant(ctx, readerID, Input{RequestID: "grant-" + suffix, DurationDays: "30", Remark: "活动赠送"})
	if err != nil || first.ID == "" || first.Permanent != "false" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	retry, err := service.Grant(ctx, readerID, Input{RequestID: "grant-" + suffix, DurationDays: "30", Remark: "重复请求"})
	if err != nil || retry.ID != first.ID {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	second, err := service.Grant(ctx, readerID, Input{RequestID: "grant-2-" + suffix, Permanent: true, Remark: "永久赠送"})
	if err != nil || second.ID == first.ID || second.Permanent != "true" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_membership_grants WHERE reader_id=$1 AND source_type='admin'`, readerID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	var productID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO commerce_products(product_type,product_name,price_coin,duration_days,sale_status,source_type,source_ref) VALUES('membership','后台发放30天',0,30,'off_sale','manual',$1) RETURNING id`, "membership-admin-"+suffix).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_products WHERE id=$1`, productID)
	})
	fromProduct, err := service.Grant(ctx, readerID, Input{ProductID: fmt.Sprintf("%d", productID), Remark: "按商品发放"})
	if err != nil || fromProduct.ID == "" || fromProduct.Permanent != "false" || fromProduct.SourceRef == "" || fromProduct.ID == first.ID {
		t.Fatalf("product grant=%+v err=%v", fromProduct, err)
	}
	if _, err := service.Grant(ctx, readerID, Input{ProductID: fmt.Sprintf("%d", readerID+999999), Remark: "不存在商品"}); err == nil {
		t.Fatal("expected missing product error")
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_membership_grants WHERE reader_id=$1 AND source_type='admin'`, readerID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("after product grant count=%d err=%v", count, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_accounts SET status='disabled' WHERE id=$1`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Grant(ctx, readerID, Input{RequestID: "grant-disabled-" + suffix, DurationDays: "30", Remark: "禁用账号"}); !errors.Is(err, readercontract.ErrAccountDisabled) || apperror.Expose(err).Code != apperror.CodeConflict {
		t.Fatalf("disabled grant err=%v", err)
	}
	if _, err := service.Grant(ctx, readerID+999, Input{RequestID: "grant-missing-" + suffix, DurationDays: "30", Remark: "不存在账号"}); !errors.Is(err, readercontract.ErrAccountNotFound) || apperror.Expose(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing grant err=%v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_membership_grants WHERE reader_id=$1 AND source_type='admin'`, readerID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("disabled grant changed facts count=%d err=%v", count, err)
	}
}
