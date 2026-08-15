package adminuser

import (
	"context"
	"errors"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/accountsync"
)

type ProjectionConsistency interface {
	Check(context.Context, bool) (accountsync.Report, error)
}

type Service struct {
	Repo  Repository
	tx    Transactor
	sync  commercecontract.ReaderSearchProjectionWriter
	check ProjectionConsistency
}

func NewService(repo Repository, transactor Transactor, projectionSync commercecontract.ReaderSearchProjectionWriter, consistency ProjectionConsistency) *Service {
	return &Service{Repo: repo, tx: transactor, sync: projectionSync, check: consistency}
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
	return s.Repo.List(ctx, k, st, p, n)
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
