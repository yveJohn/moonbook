//go:build integration

package adminrecharge

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"sync"
	"testing"
	"time"
)

func TestRechargeProductAdminCrud(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	v, e := r.Create(ctx, Input{ProductName: "管理集成档位", DiamondAmount: "123", PriceUSDT: "12.30", SaleStatus: "off_sale", SortOrder: 99})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_products WHERE id=$1`, v.ID)
	})
	if v.ID <= 0 || v.DiamondAmount != 123 {
		t.Fatalf("created=%+v", v)
	}
	updated, e := r.Update(ctx, v.ID, Input{ProductName: "更新档位", DiamondAmount: "456", PriceUSDT: "45.60", SaleStatus: "on_sale", SortOrder: 1})
	if e != nil || updated.ProductName != "更新档位" {
		t.Fatalf("updated=%+v err=%v", updated, e)
	}
	rows, total, e := r.List(ctx, "更新", 1, 20)
	if e != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, e)
	}
	if e = r.Delete(ctx, v.ID); e != nil {
		t.Fatal(e)
	}
}

func TestRechargeProductAdminCreateUsesSequence(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	const count = 8
	ids := make(chan int64, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := r.Create(ctx, Input{ProductName: "并发档位", DiamondAmount: "100", PriceUSDT: "1.00", SaleStatus: "off_sale", SortOrder: i})
			if err != nil {
				errs <- err
				return
			}
			ids <- v.ID
		}(i)
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %d", id)
		}
		seen[id] = true
		defer func(id int64) {
			_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_products WHERE id=$1`, id)
		}(id)
	}
	if len(seen) != count {
		t.Fatalf("created %d products, want %d", len(seen), count)
	}
}

func TestRechargeSettingAdminReadAndUpdate(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	original, err := r.GetSetting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = r.UpdateSetting(context.Background(), SettingInput{CustomEnabled: original.CustomEnabled, DiamondsPerUSDT: original.DiamondsPerUSDT, MinDiamondAmount: original.MinDiamondAmount, MaxDiamondAmount: original.MaxDiamondAmount})
	})
	updated, err := r.UpdateSetting(ctx, SettingInput{CustomEnabled: false, DiamondsPerUSDT: "7.125", MinDiamondAmount: "9", MaxDiamondAmount: "90000"})
	if err != nil || updated.ID != 1 || updated.CustomEnabled || updated.DiamondsPerUSDT != "7.125" || updated.MinDiamondAmount != "9" || updated.MaxDiamondAmount != "90000" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	for _, in := range []SettingInput{
		{CustomEnabled: true, DiamondsPerUSDT: "0", MinDiamondAmount: "1", MaxDiamondAmount: "2"},
		{CustomEnabled: true, DiamondsPerUSDT: "7", MinDiamondAmount: "10", MaxDiamondAmount: "9"},
		{CustomEnabled: true, DiamondsPerUSDT: "7.123456789", MinDiamondAmount: "1", MaxDiamondAmount: "2"},
	} {
		if _, err := r.UpdateSetting(ctx, in); err == nil {
			t.Fatalf("expected invalid setting for %+v", in)
		}
	}
}
