package adminuser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/accountsync"
)

type ProjectionConsistency interface {
	Check(context.Context, bool) (accountsync.Report, error)
}

type Service struct {
	Repo    Repository
	tx      Transactor
	sync    commercecontract.ReaderSearchProjectionWriter
	check   ProjectionConsistency
	wallets commercecontract.WalletReader
}

func NewService(repo Repository, transactor Transactor, projectionSync commercecontract.ReaderSearchProjectionWriter, consistency ProjectionConsistency, wallets commercecontract.WalletReader) *Service {
	return &Service{Repo: repo, tx: transactor, sync: projectionSync, check: consistency, wallets: wallets}
}

func (s *Service) CheckProjectionConsistency(ctx context.Context, repair bool) (accountsync.Report, error) {
	if s == nil || s.check == nil {
		return accountsync.Report{}, errors.New("reader projection consistency service is unavailable")
	}
	return s.check.Check(ctx, repair)
}
func (s *Service) List(ctx context.Context, k, st string, p, n int) ([]User, int64, error) {
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	rows, total, err := s.Repo.List(ctx, k, st, p, n)
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachBalances(ctx, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) attachBalances(ctx context.Context, rows []User) error {
	for i := range rows {
		if rows[i].RechargeBalance == "" {
			rows[i].RechargeBalance = "0"
		}
		if rows[i].BonusBalance == "" {
			rows[i].BonusBalance = "0"
		}
	}
	if s == nil || s.wallets == nil || len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	index := make(map[int64]int, len(rows))
	for i, row := range rows {
		id, err := strconv.ParseInt(row.ID, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
		index[id] = i
	}
	if len(ids) == 0 {
		return nil
	}
	wallets, err := s.wallets.Wallets(ctx, ids)
	if err != nil {
		return err
	}
	for _, wallet := range wallets {
		i, ok := index[wallet.ReaderID]
		if !ok {
			continue
		}
		rows[i].RechargeBalance = strconv.FormatInt(wallet.RechargeCoinBalance, 10)
		rows[i].BonusBalance = strconv.FormatInt(wallet.BonusCoinBalance, 10)
	}
	return nil
}
func (s *Service) Get(ctx context.Context, id int64) (User, error) { return s.Repo.Get(ctx, id) }
func (s *Service) SetStatus(ctx context.Context, id int64, st string) (User, error) {
	if s.Repo == nil || s.tx == nil || s.sync == nil {
		return User{}, errors.New("reader status service is unavailable")
	}
	var user User
	err := s.tx.Within(ctx, func(txCtx context.Context) error {
		updated, err := s.Repo.SetStatus(txCtx, id, st)
		if err != nil {
			return err
		}
		if err := s.sync.UpsertReaderSearchProjection(txCtx, commercecontract.ReaderSearchProjection{
			ReaderID: id,
			Username: updated.Username,
			Nickname: updated.Nickname,
			Status:   updated.Status,
		}); err != nil {
			return err
		}
		user = updated
		return nil
	})
	return user, err
}
func (s *Service) ResetPassword(ctx context.Context, id int64, password, confirm string) error {
	if len([]rune(password)) < 6 || len([]rune(password)) > 64 || password != confirm {
		return errors.New("invalid reader password")
	}
	return s.Repo.ResetPassword(ctx, id, password, confirm)
}

func (s *Service) ListOperations(ctx context.Context, id int64, p, n int) ([]Operation, int64, error) {
	if id <= 0 {
		return nil, 0, errors.New("invalid reader id")
	}
	if _, err := s.Repo.Get(ctx, id); err != nil {
		return nil, 0, err
	}
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	rows, total, err := s.Repo.ListOperations(ctx, id, p, n)
	if err != nil {
		return nil, 0, err
	}
	for i := range rows {
		rows[i].OperatorName = operatorLabel(rows[i].OperatorUsername, rows[i].OperatorNickname)
		rows[i].Action, rows[i].Summary = describeOperation(rows[i].Method, rows[i].Path, rows[i].Body)
		if rows[i].Summary == "" {
			rows[i].Summary = strings.TrimSpace(rows[i].ErrorMessage)
		}
		rows[i].Body = ""
	}
	return rows, total, nil
}

func operatorLabel(username, nickname string) string {
	switch {
	case username != "" && nickname != "":
		return username + "（" + nickname + "）"
	case username != "":
		return username
	default:
		return nickname
	}
}

func describeOperation(method, path, body string) (string, string) {
	switch {
	case strings.HasSuffix(path, "/password"):
		return "重置密码", ""
	case strings.HasSuffix(path, "/status"):
		payload := parseOperationBody(body)
		summary := ""
		switch payload["status"] {
		case "enabled":
			summary = "启用"
		case "disabled":
			summary = "停用"
		}
		return "启停账号", summary
	case strings.HasSuffix(path, "/membership"):
		payload := parseOperationBody(body)
		summary := strings.TrimSpace(payload["remark"])
		if summary == "" {
			summary = strings.TrimSpace(payload["productId"])
		}
		return "发放会员", summary
	case strings.HasSuffix(path, "/adjust"):
		payload := parseOperationBody(body)
		coin := coinLabel(payload["coinType"])
		amount := strings.TrimSpace(payload["amount"])
		reason := strings.TrimSpace(payload["reason"])
		verb := "发放"
		if payload["direction"] == "expense" {
			verb = "扣减"
		}
		action := "调整钱包"
		if coin != "" {
			action = verb + coin
		}
		parts := make([]string, 0, 2)
		if coin != "" && amount != "" {
			parts = append(parts, coin+amount)
		}
		if reason != "" {
			parts = append(parts, reason)
		}
		return action, strings.Join(parts, " / ")
	default:
		return strings.TrimSpace(method + " " + path), ""
	}
}

func coinLabel(value string) string {
	switch value {
	case "recharge":
		return "钻石"
	case "bonus":
		return "金币"
	default:
		return ""
	}
}

func parseOperationBody(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var generic map[string]any
	if json.Unmarshal([]byte(raw), &generic) != nil {
		return out
	}
	for key, value := range generic {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") {
			continue
		}
		if value == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			continue
		}
		out[key] = text
	}
	return out
}
