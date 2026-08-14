package auth

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type RegisterRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	InviteCode string `json:"inviteCode"`
}
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type PasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}
type LoginResponse struct {
	AccessToken string          `json:"accessToken"`
	ExpireIn    int64           `json:"expireIn"`
	Reader      ProfileResponse `json:"reader"`
}
type ProfileResponse struct {
	ReaderID string `json:"readerId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Status   string `json:"status"`
}

// RegistrationService is implemented by the invite transaction service. It is
// kept separate so authentication remains usable in tests and during rollout.
type RegistrationService interface {
	Register(context.Context, RegisterRequest) (ReaderAccount, AccessToken, error)
}

type Handler struct {
	service      *Service
	registration RegistrationService
}

func NewHandler(service *Service, registration RegistrationService) *Handler {
	return &Handler{service: service, registration: registration}
}

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	auth := group.Group("/reader/auth")
	auth.POST("/register", handler.register)
	auth.POST("/login", handler.login)
	auth.POST("/logout", RequireReader(handler.service), handler.logout)
	auth.GET("/profile", RequireReader(handler.service), handler.profile)
	auth.PUT("/password", RequireReader(handler.service), handler.password)
}

func (h *Handler) register(c *gin.Context) {
	var req RegisterRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Username) == "" || req.Password == "" || strings.TrimSpace(req.InviteCode) == "" {
		readerError(c, ErrInvalidCredentials)
		return
	}
	if h.registration == nil {
		readerError(c, ErrAuthUnavailable)
		return
	}
	account, token, err := h.registration.Register(c.Request.Context(), req)
	if err != nil {
		readerError(c, err)
		return
	}
	readerOK(c, LoginResponse{AccessToken: token.AccessToken, ExpireIn: int64(token.ExpiresAt.Sub(time.Now()).Seconds()), Reader: profileOf(account)}, "操作成功")
}
func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if c.ShouldBindJSON(&req) != nil {
		readerError(c, ErrInvalidCredentials)
		return
	}
	token, err := h.service.Login(c.Request.Context(), req.Username, req.Password, c.ClientIP())
	if err != nil {
		readerError(c, err)
		return
	}
	account, accountErr := h.service.repo.FindAccountByUsername(c.Request.Context(), req.Username)
	if accountErr != nil {
		readerError(c, ErrInvalidSession)
		return
	}
	readerOK(c, LoginResponse{AccessToken: token.AccessToken, ExpireIn: int64(token.ExpiresAt.Sub(time.Now()).Seconds()), Reader: profileOf(account)}, "操作成功")
}
func (h *Handler) logout(c *gin.Context) {
	raw, _ := c.Get("reader.token")
	if err := h.service.Logout(c.Request.Context(), raw.(string)); err != nil {
		readerError(c, err)
		return
	}
	readerOK(c, nil, "操作成功")
}
func (h *Handler) profile(c *gin.Context) {
	identity, _ := ReaderIdentity(c)
	account, err := h.service.repo.FindAccountByID(c.Request.Context(), identity.ReaderID)
	if err != nil {
		readerError(c, ErrInvalidSession)
		return
	}
	readerOK(c, profileOf(account), "查询成功")
}
func (h *Handler) password(c *gin.Context) {
	var req PasswordRequest
	if c.ShouldBindJSON(&req) != nil || req.NewPassword == "" || req.NewPassword != req.ConfirmPassword {
		readerError(c, ErrInvalidCredentials)
		return
	}
	raw, _ := c.Get("reader.token")
	if err := h.service.ChangePassword(c.Request.Context(), raw.(string), req.CurrentPassword, req.NewPassword); err != nil {
		readerError(c, err)
		return
	}
	readerOK(c, nil, "操作成功")
}

func profileOf(account ReaderAccount) ProfileResponse {
	return ProfileResponse{ReaderID: strconv.FormatInt(account.ID, 10), Username: account.Username, Nickname: account.Nickname, Status: account.Status}
}
