package epusdt

import "testing"

func TestSignatureMatchesFrozenImplementationVector(t *testing.T) {
	fields := map[string]string{
		"token":     "usdt",
		"amount":    "1.00",
		"name":      "钻石充值",
		"empty":     "",
		"signature": "ignored",
		"pid":       "moonbook",
	}
	wantCanonical := "amount=1.00&name=钻石充值&pid=moonbook&token=usdt"
	if got := Canonical(fields); got != wantCanonical {
		t.Fatalf("canonical=%q want=%q", got, wantCanonical)
	}
	const wantSignature = "61fbae54560a0b6c1e5e8ce484bfb23420583b4bc87f8791ce571dc83c27f824"
	if got := Sign(fields, "test-secret"); got != wantSignature {
		t.Fatalf("signature=%q want=%q", got, wantSignature)
	}
	if !Verify(fields, wantSignature, "test-secret") {
		t.Fatal("valid signature rejected")
	}
}

func TestVerifyRejectsMalformedOrIncorrectSignatures(t *testing.T) {
	fields := map[string]string{"amount": "1.00", "pid": "moonbook"}
	signature := Sign(fields, "test-secret")
	for _, candidate := range []string{"", "abc", signature[:63], "61FBAE54560A0B6C1E5E8CE484BFB23420583B4BC87F8791CE571DC83C27F824", signature[:63] + "g"} {
		if Verify(fields, candidate, "test-secret") {
			t.Fatalf("malformed signature accepted: %q", candidate)
		}
	}
	if Verify(fields, signature, "wrong-secret") {
		t.Fatal("signature made with another secret accepted")
	}
}
