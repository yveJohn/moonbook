package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/reconcile"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"strings"
	"time"
)

func main() { os.Exit(run()) }
func run() int {
	dsn := strings.TrimSpace(os.Getenv("MOONBOOK_DATABASE_DSN"))
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "MOONBOOK_DATABASE_DSN is required")
		return 2
	}
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	r, e := reconcile.Full(ctx, db)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	fmt.Printf("wallet_checked=%d recharge_orders=%d purchase_orders=%d callbacks=%d membership_grants=%d entitlements=%d mismatches=%d\n", r.Wallets.Checked, r.RechargeOrders, r.PurchaseOrders, r.Callbacks, r.MembershipGrants, r.Entitlements, len(r.Mismatches))
	for _, m := range r.Mismatches {
		fmt.Printf("domain=%s key=%s field=%s expected=%s actual=%s\n", m.Domain, m.Key, m.Field, m.Expected, m.Actual)
	}
	if len(r.Mismatches) > 0 {
		return 1
	}
	return 0
}
