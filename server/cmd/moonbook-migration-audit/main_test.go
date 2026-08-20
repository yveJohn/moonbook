package main

import (
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/reconcile"
)

func TestFinanceReportHashesBusinessKeys(t *testing.T) {
	report := financeReport(reconcile.FullReport{Mismatches: []reconcile.DomainMismatch{{Domain: "wallet", Key: "9007199254740993", Field: "bonus.balance"}}})
	if report.MismatchCount != 1 || len(report.Samples) != 1 || len(report.Samples[0].Fingerprint) != 64 {
		t.Fatalf("report=%+v", report)
	}
	if strings.Contains(report.Samples[0].Fingerprint, "9007199254740993") {
		t.Fatal("finance sample exposed a business key")
	}
}
