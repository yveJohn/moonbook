package me

import (
	"encoding/json"
	"net/http"
	"strconv"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}
type pageResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Rows  any    `json:"rows"`
	Total int64  `json:"total"`
}
type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func RegisterRoutes(group *gin.RouterGroup, service *Service, auth *readerauth.Service) {
	h := NewHandler(service)
	r := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	r.GET("/bookshelf", h.listBookshelf)
	r.POST("/bookshelf/:bookId", h.addBookshelf)
	r.DELETE("/bookshelf/:bookId", h.removeBookshelf)
	r.GET("/likes", h.listLikes)
	r.POST("/likes/:bookId", h.like)
	r.DELETE("/likes/:bookId", h.unlike)
	r.POST("/feedbacks", h.createFeedback)
	r.GET("/feedbacks", h.listFeedbacks)
	r.GET("/history", h.listHistory)
	r.GET("/history/:bookId", h.getHistory)
	r.PUT("/history/:bookId", h.updateHistory)
	r.GET("/preference", h.preference)
	r.PUT("/preference", h.updatePreference)
}
func id(c *gin.Context) (int64, error) {
	x, ok := readerauth.ReaderIdentity(c)
	if !ok {
		return 0, readerauth.ErrInvalidSession
	}
	return x.ReaderID, nil
}
func fail(c *gin.Context, e error) {
	p := apperror.Expose(e)
	code := 500
	if p.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	if p.Code == apperror.CodeInvalidArgument || p.Code == apperror.CodeNotFound {
		code = 200
	}
	c.JSON(http.StatusOK, response{Code: code, Msg: p.Message})
}
func ok(c *gin.Context, data any, msg string) {
	c.JSON(http.StatusOK, map[string]any{"code": 200, "msg": msg, "data": data})
}
func (h *Handler) listBookshelf(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.ListBookshelf(c, rid)
	if e != nil {
		fail(c, e)
		return
	}
	out := make([]map[string]any, 0, len(v))
	for _, x := range v {
		m := map[string]any{"bookshelfId": strconv.FormatInt(x.ID, 10), "bookId": strconv.FormatInt(x.BookID, 10), "bookName": x.BookName, "authorName": x.AuthorName, "lastChapterId": nil, "lastChapterName": nil, "lastReadTime": x.LastReadAt}
		if x.LastChapterID != nil {
			m["lastChapterId"] = strconv.FormatInt(*x.LastChapterID, 10)
		}
		if x.LastChapterName != nil {
			m["lastChapterName"] = *x.LastChapterName
		}
		out = append(out, m)
	}
	ok(c, out, "查询成功")
}
func (h *Handler) addBookshelf(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	bid, e := ParseID(c.Param("bookId"))
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.AddBookshelf(c, rid, bid)
	if e != nil {
		fail(c, e)
		return
	}
	m := map[string]any{"bookshelfId": strconv.FormatInt(v.ID, 10), "bookId": strconv.FormatInt(v.BookID, 10), "bookName": v.BookName, "authorName": v.AuthorName, "lastChapterId": nil, "lastChapterName": nil, "lastReadTime": v.LastReadAt}
	if v.LastChapterID != nil {
		m["lastChapterId"] = strconv.FormatInt(*v.LastChapterID, 10)
	}
	if v.LastChapterName != nil {
		m["lastChapterName"] = *v.LastChapterName
	}
	ok(c, m, "操作成功")
}
func (h *Handler) removeBookshelf(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	bid, e := ParseID(c.Param("bookId"))
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.RemoveBookshelf(c, rid, bid)
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, v, "操作成功")
}
func (h *Handler) listLikes(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.ListLikes(c, rid)
	if e != nil {
		fail(c, e)
		return
	}
	out := make([]map[string]any, 0, len(v))
	for _, x := range v {
		out = append(out, map[string]any{"likeId": strconv.FormatInt(x.BookLikeID, 10), "bookId": strconv.FormatInt(x.BookID, 10), "bookName": x.BookName, "authorName": x.AuthorName, "bookDesc": x.Description, "categoryCode": x.CategoryCode, "categoryName": x.CategoryName, "wordCount": x.WordCount, "likeCount": x.LikeCount, "likedAt": x.LikedAt})
	}
	ok(c, out, "查询成功")
}
func (h *Handler) like(c *gin.Context)   { h.likeChange(c, true) }
func (h *Handler) unlike(c *gin.Context) { h.likeChange(c, false) }
func (h *Handler) likeChange(c *gin.Context, liked bool) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	bid, e := ParseID(c.Param("bookId"))
	if e != nil {
		fail(c, e)
		return
	}
	var v BookLike
	if liked {
		v, e = h.service.Like(c, rid, bid)
	} else {
		v, e = h.service.Unlike(c, rid, bid)
	}
	if e != nil {
		fail(c, e)
		return
	}
	m := map[string]any{"likeId": nil, "bookId": strconv.FormatInt(v.BookID, 10), "liked": v.Liked, "likeCount": v.LikeCount}
	if v.ID > 0 {
		m["likeId"] = strconv.FormatInt(v.ID, 10)
	}
	ok(c, m, "操作成功")
}

type feedbackRequest struct {
	Content string `json:"content"`
}

func feedbackMap(v Feedback) map[string]any {
	reply := any(nil)
	if v.Reply != nil && *v.Reply != "" {
		reply = *v.Reply
	}
	return map[string]any{"id": strconv.FormatInt(v.ID, 10), "content": v.Content, "status": v.Status, "replyContent": reply, "replyTime": v.RepliedAt, "createTime": v.CreatedAt}
}
func (h *Handler) createFeedback(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	var req feedbackRequest
	if c.ShouldBindJSON(&req) != nil {
		fail(c, ErrInvalidInput)
		return
	}
	v, e := h.service.CreateFeedback(c, rid, req.Content)
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, feedbackMap(v), "操作成功")
}
func (h *Handler) listFeedbacks(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	p, _ := strconv.Atoi(c.Query("pageNum"))
	s, _ := strconv.Atoi(c.Query("pageSize"))
	v, total, e := h.service.ListFeedbacks(c, rid, p, s)
	if e != nil {
		fail(c, e)
		return
	}
	out := make([]map[string]any, 0, len(v))
	for _, x := range v {
		out = append(out, feedbackMap(x))
	}
	c.JSON(http.StatusOK, pageResponse{Code: 200, Msg: "查询成功", Rows: out, Total: total})
}
func historyMap(v History) map[string]any {
	return map[string]any{"historyId": strconv.FormatInt(v.ID, 10), "bookId": strconv.FormatInt(v.BookID, 10), "chapterId": strconv.FormatInt(v.ChapterID, 10), "chapterNo": v.ChapterNo, "chapterName": v.ChapterName, "positionType": v.PositionType, "positionValue": v.PositionValue, "progressPercent": v.ProgressPercent, "lastReadTime": v.LastReadAt}
}
func (h *Handler) listHistory(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.ListHistory(c, rid)
	if e != nil {
		fail(c, e)
		return
	}
	out := make([]map[string]any, 0, len(v))
	for _, x := range v {
		out = append(out, historyMap(x))
	}
	ok(c, out, "查询成功")
}
func (h *Handler) getHistory(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	bid, e := ParseID(c.Param("bookId"))
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.GetHistory(c, rid, bid)
	if e != nil {
		fail(c, e)
		return
	}
	if v == nil {
		ok(c, nil, "查询成功")
		return
	}
	ok(c, historyMap(*v), "查询成功")
}

type historyRequest struct {
	ChapterID       string          `json:"chapterId"`
	ChapterNo       *int            `json:"chapterNo"`
	PositionType    string          `json:"positionType"`
	PositionValue   int             `json:"positionValue"`
	ProgressPercent json.RawMessage `json:"progressPercent"`
}

func (h *Handler) updateHistory(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	bid, e := ParseID(c.Param("bookId"))
	if e != nil {
		fail(c, e)
		return
	}
	var req historyRequest
	if c.ShouldBindJSON(&req) != nil {
		fail(c, ErrInvalidInput)
		return
	}
	cid, e := ParseID(req.ChapterID)
	if e != nil {
		fail(c, e)
		return
	}
	p, e := parseProgress(req.ProgressPercent)
	if e != nil {
		fail(c, e)
		return
	}
	if req.PositionType == "" {
		req.PositionType = "scroll"
	}
	v, e := h.service.UpdateHistory(c, rid, bid, HistoryInput{ChapterID: cid, ChapterNo: req.ChapterNo, PositionType: req.PositionType, PositionValue: req.PositionValue, ProgressPercent: p})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, historyMap(v), "操作成功")
}
func prefMap(v Preference) map[string]any {
	m := map[string]any{"preferenceId": nil, "fontSize": v.FontSize, "lineHeight": v.LineHeight, "theme": v.Theme, "readingMode": v.ReadingMode}
	if v.ID > 0 {
		m["preferenceId"] = strconv.FormatInt(v.ID, 10)
	}
	return m
}
func (h *Handler) preference(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.GetPreference(c, rid)
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, prefMap(v), "查询成功")
}

type preferenceRequest struct {
	FontSize    *int            `json:"fontSize"`
	LineHeight  json.RawMessage `json:"lineHeight"`
	Theme       string          `json:"theme"`
	ReadingMode string          `json:"readingMode"`
}

func (h *Handler) updatePreference(c *gin.Context) {
	rid, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	var req preferenceRequest
	if c.ShouldBindJSON(&req) != nil {
		fail(c, ErrInvalidInput)
		return
	}
	lh, _ := parseProgress(req.LineHeight)
	if len(req.LineHeight) == 0 {
		lh = "1.80"
	}
	v, e := h.service.UpdatePreference(c, rid, PreferenceInput{FontSize: req.FontSize, LineHeight: lh, Theme: req.Theme, ReadingMode: req.ReadingMode})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, prefMap(v), "操作成功")
}
