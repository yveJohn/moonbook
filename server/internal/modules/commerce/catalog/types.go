package catalog

import "strconv"

type ChargeMode string

const (
	WordCharge     ChargeMode = "word_charge"
	MembershipOnly ChargeMode = "membership_only"
	LoginFree      ChargeMode = "login_free"
	FixedPrice     ChargeMode = "fixed_price"
)

type AccessReason string

const (
	LoginRequired           AccessReason = "login_required"
	BookOwned               AccessReason = "book_owned"
	ChapterOwned            AccessReason = "chapter_owned"
	Membership              AccessReason = "membership"
	LoginFreeReason         AccessReason = "login_free"
	MembershipRequired      AccessReason = "membership_required"
	BookPurchaseRequired    AccessReason = "book_purchase_required"
	ChapterPurchaseRequired AccessReason = "chapter_purchase_required"
	FreeChapter             AccessReason = "free_chapter"
	UnsupportedMode         AccessReason = "unsupported_mode"
)

type Product struct {
	ID          int64
	ProductType string
	TargetID    *int64
	ProductName string
	PriceCoin   int64
	SaleStatus  string
}

type ChapterPricing struct {
	WordUnit int
	CoinUnit int64
	Enabled  bool
}

type ReaderContext struct {
	ReaderID     *int64
	BookOwned    bool
	ChapterOwned bool
	Membership   bool
}

type AccessRequest struct {
	ReaderID         *int64
	BookID           int64
	ChapterID        int64
	ChargeMode       string
	ChapterWordCount int
	FixedPriceCoin   *int64
}

type AccessContext struct {
	Product        *Product
	ChapterProduct *Product
	Pricing        ChapterPricing
	Reader         ReaderContext
}

type AccessResult struct {
	BookID             string `json:"bookId"`
	ChapterID          string `json:"chapterId,omitempty"`
	ChargeMode         string `json:"chargeMode"`
	Readable           bool   `json:"readable"`
	AccessReason       string `json:"accessReason"`
	MembershipEntitled bool   `json:"membershipEntitled"`
	BookPurchased      bool   `json:"bookPurchased"`
	ChapterPurchased   bool   `json:"chapterPurchased"`
	Purchasable        bool   `json:"purchasable"`
	ChapterWordCount   int    `json:"chapterWordCount"`
	PricingWordUnit    int    `json:"pricingWordUnit"`
	PricingCoinUnit    string `json:"pricingCoinUnit,omitempty"`
	ChapterPrice       string `json:"chapterPrice,omitempty"`
	ProductID          string `json:"productId,omitempty"`
	ProductName        string `json:"productName,omitempty"`
	PriceCoin          string `json:"priceCoin,omitempty"`
	SaleStatus         string `json:"saleStatus,omitempty"`
}

func decimal(v int64) string { return strconv.FormatInt(v, 10) }
