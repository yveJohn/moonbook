package reconcile

import "testing"

func TestReportShape(t *testing.T) {
	r := Report{Checked: 1, Mismatches: []Mismatch{{ReaderID: 1, CoinType: "bonus", Field: "balance", Expected: 2, Actual: 1}}}
	if len(r.Mismatches) != 1 || r.Mismatches[0].Expected == r.Mismatches[0].Actual {
		t.Fatal("mismatch should remain actionable")
	}
}
