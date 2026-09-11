package adminuser

import (
	"context"
	"errors"
	"strconv"

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
