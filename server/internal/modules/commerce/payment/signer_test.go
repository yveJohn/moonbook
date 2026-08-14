package payment

import "testing"

func TestEpusdtSignatureCanonicalAndConstantComparison(t *testing.T) {
	fields := map[string]string{"pid": "merchant", "amount": "2.00", "order_id": "MBR1", "signature": "ignored", "empty": ""}
	sig := Sign(fields, "secret")
	if !Verify(fields, sig, "secret") {
		t.Fatal("valid signature rejected")
	}
	if Verify(fields, sig, "wrong") {
		t.Fatal("wrong secret accepted")
	}
	if Canonical(fields) != "amount=2.00&order_id=MBR1&pid=merchant" {
		t.Fatalf("canonical=%q", Canonical(fields))
	}
}
