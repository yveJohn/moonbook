package adminpayment

import "testing"

func TestNormalizeBaseURLAndDeriveEndpoints(t *testing.T) {
	epusdt, err := normalizeBaseURL(" https://pay.example:8443/ ")
	if err != nil || epusdt != "https://pay.example:8443" {
		t.Fatalf("epusdt=%q err=%v", epusdt, err)
	}
	endpoints, err := deriveEndpoints(epusdt, "https://reader.example/")
	if err != nil {
		t.Fatal(err)
	}
	want := Endpoints{
		CreateURL:   "https://pay.example:8443/payments/gmpay/v1/order/create-transaction",
		NotifyURL:   "https://reader.example/prod-api/reader/payment/epusdt/notify",
		RedirectURL: "https://reader.example/me/recharge",
		HealthURL:   "https://pay.example:8443/",
		SyncURL:     "https://pay.example:8443/pay/check-status/{trade_id}",
	}
	if endpoints != want {
		t.Fatalf("endpoints=%+v want=%+v", endpoints, want)
	}
}

func TestNormalizeBaseURLRejectsNonOriginValues(t *testing.T) {
	for _, value := range []string{
		"", "ftp://pay.example", "https://user:pass@pay.example", "https://pay.example/api",
		"https://pay.example?mode=test", "https://pay.example#fragment", "//pay.example",
	} {
		t.Run(value, func(t *testing.T) {
			if normalized, err := normalizeBaseURL(value); err == nil {
				t.Fatalf("normalizeBaseURL(%q)=%q, want error", value, normalized)
			}
		})
	}
}
