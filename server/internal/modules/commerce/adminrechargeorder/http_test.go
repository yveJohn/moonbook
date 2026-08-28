package adminrechargeorder

import "testing"

func TestRenderIncludesFailureDiagnosticsAndStringIDs(t *testing.T) {
	value := render(Order{
		ID:             "9223372036854775807",
		ReaderID:       "9007199254740993",
		FailureCode:    "RESPONSE_CURRENCY_MISMATCH",
		FailureMessage: "EPUSDT create response currency did not match the local order",
	})
	if value["id"] != "9223372036854775807" || value["readerId"] != "9007199254740993" {
		t.Fatalf("long IDs changed: %+v", value)
	}
	if value["failureCode"] != "RESPONSE_CURRENCY_MISMATCH" || value["failureMessage"] != "EPUSDT create response currency did not match the local order" {
		t.Fatalf("failure diagnostics missing: %+v", value)
	}
}
