package recharge

import "testing"

func TestCeilMoneyUsesExactDecimalArithmetic(t *testing.T) {
	for _, tc := range []struct {
		diamonds   int64
		rate, want string
	}{
		{17, "7", "2.43"},
		{35, "7", "5.00"},
		{1, "3.33333333", "0.31"},
		{7, "7.00000000", "1.00"},
	} {
		got, err := ceilMoney(tc.diamonds, tc.rate)
		if err != nil || got != tc.want {
			t.Errorf("ceilMoney(%d, %q) = %q, %v; want %q", tc.diamonds, tc.rate, got, err, tc.want)
		}
	}
}

func TestCeilMoneyRejectsInvalidRate(t *testing.T) {
	if _, err := ceilMoney(10, "0"); err == nil {
		t.Fatal("zero rate must be rejected")
	}
	if _, err := ceilMoney(10, "not-a-rate"); err == nil {
		t.Fatal("invalid rate must be rejected")
	}
}
