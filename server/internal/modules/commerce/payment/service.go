package payment

import (
	"context"
	"database/sql"
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
	return s.process(ctx, callback, func(repository Repository) error {
		return repository.Process(ctx, callback)
	})
}

func (s *Service) ProcessAttempt(ctx context.Context, attemptID int64, callback Callback) error {
	if attemptID <= 0 {
		return newProcessingError(FailureTransaction, ErrInvalidAttempt)
	}
	return s.process(ctx, callback, func(repository Repository) error {
		return repository.ProcessAttempt(ctx, attemptID, callback)
	})
}

func (s *Service) process(ctx context.Context, callback Callback, process func(Repository) error) error {
	if s == nil || s.Repo == nil {
		return newProcessingError(FailureDependency, readerDependencyUnavailable)
	}
	if strings.TrimSpace(callback.OrderNo) == "" {
		return newProcessingError(FailureUnknownOrder, ErrUnauthorized)
	}
	snapshot, err := s.Repo.VerificationSnapshot(ctx, callback.OrderNo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newProcessingError(FailureUnknownOrder, ErrUnauthorized)
		}
		return newProcessingError(FailureDependency, readerDependencyUnavailable)
	}
	if s.Credentials == nil {
		return enrichProcessingError(newProcessingError(FailureUnknownCredential, ErrUnauthorized), false, snapshot.OrderID)
	}
	credential, ok := s.Credentials.Verification(snapshot.CredentialRef)
	if !ok {
		return enrichProcessingError(newProcessingError(FailureUnknownCredential, ErrUnauthorized), false, snapshot.OrderID)
	}
	expectedPID := strings.TrimSpace(snapshot.MerchantPID)
	if expectedPID == "" {
		if strings.TrimSpace(snapshot.CredentialRef) != "" {
			return enrichProcessingError(newProcessingError(FailurePIDMismatch, ErrUnauthorized), false, snapshot.OrderID)
		}
		expectedPID = credential.PID()
	}
	if credential.PID() != expectedPID || callback.PID != expectedPID || callback.Fields["pid"] != callback.PID {
		return enrichProcessingError(newProcessingError(FailurePIDMismatch, ErrUnauthorized), false, snapshot.OrderID)
	}
	if callback.Fields["order_id"] != callback.OrderNo {
		return enrichProcessingError(newProcessingError(FailureSnapshotMismatch, ErrRejected), false, snapshot.OrderID)
	}
	if callback.Fields["signature"] != callback.Signature || !Verify(callback.Fields, callback.Signature, credential.Secret()) {
		return enrichProcessingError(newProcessingError(FailureSignatureInvalid, ErrUnauthorized), false, snapshot.OrderID)
	}
	return enrichProcessingError(process(s.Repo), true, snapshot.OrderID)
}

var readerDependencyUnavailable = errors.New("payment callback dependency unavailable")
