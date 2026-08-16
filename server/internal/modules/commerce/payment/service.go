package payment

import (
	"context"
	"errors"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

var ErrUnauthorized = errors.New("payment callback unauthorized")

type Service struct {
	Repo        Repository
	Credentials *epusdt.CredentialProvider
}

func NewService(repo Repository, credentials *epusdt.CredentialProvider) *Service {
	return &Service{Repo: repo, Credentials: credentials}
}

func (s *Service) Process(ctx context.Context, callback Callback) error {
	if s == nil || s.Repo == nil || strings.TrimSpace(callback.OrderNo) == "" {
		return ErrUnauthorized
	}
	snapshot, err := s.Repo.VerificationSnapshot(ctx, callback.OrderNo)
	if err != nil {
		return err
	}
	credential, ok := s.Credentials.Verification(snapshot.CredentialRef)
	if !ok {
		return ErrUnauthorized
	}
	expectedPID := strings.TrimSpace(snapshot.MerchantPID)
	if expectedPID == "" {
		if strings.TrimSpace(snapshot.CredentialRef) != "" {
			return ErrUnauthorized
		}
		expectedPID = credential.PID()
	}
	if credential.PID() != expectedPID || callback.PID != expectedPID || callback.Fields["pid"] != callback.PID ||
		callback.Fields["order_id"] != callback.OrderNo || callback.Fields["signature"] != callback.Signature ||
		!Verify(callback.Fields, callback.Signature, credential.Secret()) {
		return ErrUnauthorized
	}
	return s.Repo.Process(ctx, callback)
}
