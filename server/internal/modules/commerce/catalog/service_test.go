package catalog

import (
	"context"
	"testing"
)

type fakeRepo struct {
	contexts map[string]AccessContext
	calls    int
}

func (f *fakeRepo) LoadAccessContext(_ context.Context, r AccessRequest) (AccessContext, error) {
	f.calls++
	return f.contexts[key(r)], nil
}
func (f *fakeRepo) LoadAccessContexts(_ context.Context, rs []AccessRequest) (map[string]AccessContext, error) {
	f.calls++
	return f.contexts, nil
}

func req(mode string, reader *int64) AccessRequest {
	return AccessRequest{ReaderID: reader, BookID: 10, ChapterID: 20, ChargeMode: mode, ChapterWordCount: 1200}
}
func TestAccessReaderMatrix(t *testing.T) {
	id := int64(7)
	c := AccessContext{Pricing: ChapterPricing{WordUnit: 1000, CoinUnit: 2, Enabled: true}, Product: &Product{ID: 99, ProductName: "整本", PriceCoin: 20, SaleStatus: "on_sale"}, ChapterProduct: &Product{ID: 100, PriceCoin: 4, SaleStatus: "on_sale"}}
	for _, tc := range []struct {
		name, mode, reason    string
		readable, purchasable bool
	}{
		{"anonymous", string(LoginFree), string(LoginRequired), false, false},
		{"login free", string(LoginFree), string(LoginFreeReason), true, false},
		{"membership", string(MembershipOnly), string(MembershipRequired), false, false},
		{"word", string(WordCharge), string(ChapterPurchaseRequired), false, true},
		{"fixed", string(FixedPrice), string(BookPurchaseRequired), false, true},
		{"unsupported", "future", string(UnsupportedMode), false, false},
	} {
		reader := &id
		if tc.name == "anonymous" {
			reader = nil
		}
		input := req(tc.mode, reader)
		got, _ := NewService(&fakeRepo{contexts: map[string]AccessContext{key(input): c}}).AccessReader(context.Background(), input)
		if got.AccessReason != tc.reason || got.Readable != tc.readable || got.Purchasable != tc.purchasable {
			t.Errorf("%s: %#v", tc.name, got)
		}
	}
}

func TestEntitlementsTakePrecedence(t *testing.T) {
	id := int64(1)
	r := req(string("future"), &id)
	c := AccessContext{Reader: ReaderContext{ReaderID: &id, BookOwned: true}}
	got, _ := NewService(&fakeRepo{contexts: map[string]AccessContext{key(r): c}}).AccessReader(context.Background(), r)
	if !got.Readable || got.AccessReason != string(BookOwned) {
		t.Fatalf("book entitlement must win: %#v", got)
	}
	c.Reader.BookOwned = false
	c.Reader.ChapterOwned = true
	got, _ = NewService(&fakeRepo{contexts: map[string]AccessContext{key(r): c}}).AccessReader(context.Background(), r)
	if !got.Readable || got.AccessReason != string(ChapterOwned) {
		t.Fatalf("chapter entitlement must win: %#v", got)
	}
}

func TestFreeChapterAndBatchBoundary(t *testing.T) {
	id := int64(1)
	r := req(string(WordCharge), &id)
	r.ChapterWordCount = 0
	f := &fakeRepo{contexts: map[string]AccessContext{key(r): AccessContext{Reader: ReaderContext{ReaderID: &id}, Pricing: ChapterPricing{Enabled: true}}}}
	g, _ := NewService(f).AccessReader(context.Background(), r)
	if !g.Readable || g.AccessReason != string(FreeChapter) {
		t.Fatalf("%#v", g)
	}
	f.calls = 0
	_, _ = NewService(f).AccessReaders(context.Background(), []AccessRequest{r, r})
	if f.calls != 1 {
		t.Fatalf("batch loader calls=%d", f.calls)
	}
}
