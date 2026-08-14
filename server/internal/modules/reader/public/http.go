package public

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	readerwire "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/wire"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
type pageResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Rows  any    `json:"rows"`
	Total int64  `json:"total"`
}
type Handler struct{ service *Service }

func RegisterRoutes(group *gin.RouterGroup, db *sql.DB, objects *objectstore.Service, auth *readerauth.Service, access *catalog.Service) {
	h := &Handler{service: NewService(db, objects, access)}
	r := group.Group("/reader").Use(readerauth.OptionalReader(auth))
	r.GET("/seo/config", h.seoConfig)
	r.GET("/seo/robots.txt", h.robots)
	r.GET("/seo/sitemap.xml", h.sitemap)
	r.GET("/seo/sitemap-books-:page.xml", h.sitemapBooks)
	r.GET("/books/featured", h.featured)
	r.GET("/books", h.books)
	r.GET("/books/random", h.random)
	r.GET("/books/categories", h.categories)
	r.GET("/books/sub-categories", h.subCategories)
	r.GET("/books/:bookId", h.book)
	r.GET("/books/:bookId/chapters", h.chapters)
	r.GET("/chapters/:chapterId", h.chapter)
}

func ok(c *gin.Context, data any, msg string) {
	c.JSON(http.StatusOK, response{Code: 200, Msg: msg, Data: data})
}
func fail(c *gin.Context, err error) {
	p := apperror.Expose(err)
	code := 500
	if p.Code == apperror.CodeNotFound {
		code = 404
	}
	if p.Code == apperror.CodeForbidden && p.HTTPStatus >= 46101 && p.HTTPStatus <= 46105 {
		code = p.HTTPStatus
	}
	c.JSON(http.StatusOK, response{Code: code, Msg: p.Message})
}
func identity(c *gin.Context) *int64 {
	if x, found := readerauth.ReaderIdentity(c); found {
		return &x.ReaderID
	}
	return nil
}
func parseID(c *gin.Context, name string) (int64, error) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, 400, "ID必须是正整数字符串")
	}
	return value, nil
}
func category(c Category) map[string]string { return map[string]string{"code": c.Code, "name": c.Name} }

func summary(book Book) map[string]any {
	subs := make([]map[string]string, 0, len(book.SubCategories))
	for _, item := range book.SubCategories {
		subs = append(subs, category(item))
	}
	featured := 0
	if book.Featured {
		featured = 1
	}
	out := map[string]any{"bookId": strconv.FormatInt(book.ID, 10), "bookName": book.Name, "authorName": book.Author, "bookDesc": book.Description, "categoryCode": book.CategoryCode, "categoryName": book.CategoryName, "subCategories": subs, "bookStatus": book.BookStatus, "wordCount": book.WordCount, "lastChapterId": nil, "lastChapterName": book.LastChapterName, "lastChapterUpdateTime": nil, "featured": featured, "featuredNote": book.FeaturedNote, "likeCount": book.LikeCount, "productStatus": map[string]any{}}
	if book.LastChapterID != nil {
		out["lastChapterId"] = strconv.FormatInt(*book.LastChapterID, 10)
	}
	if book.LastChapterUpdatedAt != nil {
		out["lastChapterUpdateTime"] = readerwire.DateTimePointer(book.LastChapterUpdatedAt)
	}
	return out
}

func productStatus(status catalog.AccessResult) map[string]any {
	value := map[string]any{
		"bookId": status.BookID, "chargeMode": status.ChargeMode, "readable": status.Readable,
		"accessReason": status.AccessReason, "entitled": status.BookPurchased || status.MembershipEntitled,
		"purchased": status.BookPurchased, "membershipEntitled": status.MembershipEntitled,
		"purchasable": status.Purchasable, "productId": nil, "productName": nil,
		"priceCoin": nil, "saleStatus": nil, "product": nil,
	}
	if status.ProductID != "" {
		value["productId"], value["productName"], value["priceCoin"], value["saleStatus"] = status.ProductID, status.ProductName, status.PriceCoin, status.SaleStatus
		value["product"] = map[string]any{
			"id": status.ProductID, "productType": "book", "targetId": status.BookID,
			"productName": status.ProductName, "priceCoin": status.PriceCoin, "allowBonusCoin": 0,
			"durationDays": nil, "saleStatus": status.SaleStatus, "sortOrder": 0, "remark": "",
			"createTime": nil, "updateTime": nil,
		}
	}
	return value
}

func history(v *History) any {
	if v == nil {
		return nil
	}
	return map[string]any{
		"historyId": strconv.FormatInt(v.ID, 10), "bookId": strconv.FormatInt(v.BookID, 10),
		"chapterId": strconv.FormatInt(v.ChapterID, 10), "chapterNo": v.ChapterNo, "chapterName": v.ChapterName,
		"positionType": v.PositionType, "positionValue": v.PositionValue, "progressPercent": v.ProgressPercent,
		"lastReadTime": readerwire.DateTime(v.LastReadAt),
	}
}

func (h *Handler) featured(c *gin.Context) {
	books, err := h.service.Featured(c)
	if err != nil {
		fail(c, err)
		return
	}
	statuses, err := h.service.BookStatuses(c, books, identity(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]map[string]any, 0, len(books))
	for _, book := range books {
		item := summary(book)
		item["productStatus"] = productStatus(statuses[book.ID])
		out = append(out, item)
	}
	ok(c, out, "查询成功")
}
func (h *Handler) random(c *gin.Context) {
	books, err := h.service.Random(c)
	if err != nil {
		fail(c, err)
		return
	}
	statuses, err := h.service.BookStatuses(c, books, identity(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]map[string]any, 0, len(books))
	for _, book := range books {
		item := summary(book)
		item["productStatus"] = productStatus(statuses[book.ID])
		out = append(out, item)
	}
	ok(c, out, "查询成功")
}
func (h *Handler) books(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("pageNum"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.Query("pageSize"))
	if size < 1 {
		size = 10
	}
	books, total, err := h.service.List(c, c.Query("keyword"), c.Query("categoryCode"), c.Query("subCategoryCode"), page, size)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]map[string]any, 0, len(books))
	for _, book := range books {
		item := summary(book)
		delete(item, "authorName")
		delete(item, "bookDesc")
		delete(item, "bookStatus")
		delete(item, "lastChapterId")
		delete(item, "lastChapterName")
		delete(item, "featured")
		delete(item, "featuredNote")
		delete(item, "productStatus")
		out = append(out, item)
	}
	c.JSON(http.StatusOK, pageResponse{Code: 200, Msg: "查询成功", Rows: out, Total: total})
}
func (h *Handler) categories(c *gin.Context)    { h.listCategories(c, "primary") }
func (h *Handler) subCategories(c *gin.Context) { h.listCategories(c, "sub") }
func (h *Handler) listCategories(c *gin.Context, kind string) {
	items, err := h.service.Categories(c, kind)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]map[string]string, 0, len(items))
	for _, item := range items {
		out = append(out, category(item))
	}
	ok(c, out, "查询成功")
}
func (h *Handler) book(c *gin.Context) {
	id, err := parseID(c, "bookId")
	if err != nil {
		fail(c, err)
		return
	}
	detail, err := h.service.Detail(c, id, identity(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := summary(detail.Book)
	out["visitCount"] = detail.VisitCount
	out["liked"] = detail.Liked
	out["inBookshelf"] = detail.InBookshelf
	out["readingHistory"] = history(detail.History)
	out["productStatus"] = productStatus(detail.Status)
	ok(c, out, "查询成功")
}

func (h *Handler) chapters(c *gin.Context) {
	id, err := parseID(c, "bookId")
	if err != nil {
		fail(c, err)
		return
	}
	chapters, err := h.service.Chapters(c, id, identity(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]map[string]any, 0, len(chapters))
	for _, chapter := range chapters {
		access := map[string]any{"bookId": strconv.FormatInt(chapter.BookID, 10), "chapterId": strconv.FormatInt(chapter.ID, 10), "chargeMode": chapter.ChargeMode, "readable": !chapter.IsVIP, "accessReason": "free_chapter", "membershipEntitled": false, "bookPurchased": false, "chapterPurchased": false, "purchasable": chapter.IsVIP, "chapterWordCount": chapter.WordCount, "pricingWordUnit": nil, "pricingCoinUnit": nil, "chapterPrice": strconv.FormatInt(chapter.Price, 10)}
		if chapter.Access.BookID != "" {
			access = map[string]any{"bookId": chapter.Access.BookID, "chapterId": chapter.Access.ChapterID, "chargeMode": chapter.Access.ChargeMode, "readable": chapter.Access.Readable, "accessReason": chapter.Access.AccessReason, "membershipEntitled": chapter.Access.MembershipEntitled, "bookPurchased": chapter.Access.BookPurchased, "chapterPurchased": chapter.Access.ChapterPurchased, "purchasable": chapter.Access.Purchasable, "chapterWordCount": chapter.Access.ChapterWordCount, "pricingWordUnit": chapter.Access.PricingWordUnit, "pricingCoinUnit": chapter.Access.PricingCoinUnit, "chapterPrice": chapter.Access.ChapterPrice}
		}
		out = append(out, map[string]any{"chapterId": strconv.FormatInt(chapter.ID, 10), "bookId": strconv.FormatInt(chapter.BookID, 10), "chapterNo": chapter.No, "chapterName": chapter.Name, "wordCount": chapter.WordCount, "updateTime": readerwire.DateTime(chapter.UpdatedAt), "accessStatus": access, "productStatus": productStatus(normalizeBookStatus(chapter.Access))})
	}
	ok(c, out, "查询成功")
}
func (h *Handler) chapter(c *gin.Context) {
	id, err := parseID(c, "chapterId")
	if err != nil {
		fail(c, err)
		return
	}
	chapter, text, object, err := h.service.Chapter(c, id, identity(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := map[string]any{"chapterId": strconv.FormatInt(chapter.ID, 10), "bookId": strconv.FormatInt(chapter.BookID, 10), "chapterNo": chapter.No, "chapterName": chapter.Name, "bookName": chapter.BookName, "content": text, "prevChapterId": nil, "nextChapterId": nil, "contentVersion": object.Version, "contentSha256": object.SHA256, "contentBytes": object.ByteSize}
	if chapter.PrevID != nil {
		out["prevChapterId"] = strconv.FormatInt(*chapter.PrevID, 10)
	}
	if chapter.NextID != nil {
		out["nextChapterId"] = strconv.FormatInt(*chapter.NextID, 10)
	}
	ok(c, out, "查询成功")
}

func (h *Handler) seoConfig(c *gin.Context) {
	v, err := h.service.SEO(c)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, map[string]any{"id": strconv.FormatInt(v.ID, 10), "seoEnabled": v.SEOEnabled, "indexingEnabled": v.IndexingEnabled, "sitemapEnabled": v.SitemapEnabled, "siteName": v.SiteName, "siteUrl": v.SiteURL, "defaultDescription": v.DefaultDescription, "homeTitle": v.HomeTitle, "homeDescription": v.HomeDescription, "booksTitleTemplate": v.BooksTitleTemplate, "booksDescriptionTemplate": v.BooksDescriptionTemplate, "bookTitleTemplate": v.BookTitleTemplate, "bookDescriptionTemplate": v.BookDescriptionTemplate}, "查询成功")
}
func (h *Handler) robots(c *gin.Context) {
	body, err := h.service.Robots(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.Data(200, "text/plain; charset=utf-8", []byte(body))
}
func (h *Handler) sitemap(c *gin.Context) { h.writeSitemap(c, 0) }
func (h *Handler) sitemapBooks(c *gin.Context) {
	page, err := strconv.Atoi(c.Param("page"))
	if err != nil || page < 1 {
		c.Status(404)
		return
	}
	h.writeSitemap(c, page)
}
func (h *Handler) writeSitemap(c *gin.Context, page int) {
	body, err := h.service.Sitemap(c, page)
	if errors.Is(err, sql.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	c.Data(200, "application/xml; charset=utf-8", []byte(body))
}
