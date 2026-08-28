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
	Repo            Repository
	Credentials     *epusdt.CredentialProvider
	LoadCredentials CredentialLoader
}

type CredentialLoader func(context.Context) (*epusdt.CredentialProvider, error)

func NewService(repo Repository, credentials *epusdt.CredentialProvider) *Service {
	return &Service{Repo: repo, Credentials: credentials}
}

func NewDynamicService(repo Repository, loader CredentialLoader) *Service {
	return &Service{Repo: repo, LoadCredentials: loader}
}

func (s *Service) Process(ctx context.Context, callback Callback) error {
	snapshot, err := s.verify(ctx, callback)
	if err != nil {
		return err
	}
	return enrichProcessingError(s.Repo.Process(ctx, callback), true, snapshot.OrderID)
}

func (s *Service) ProcessAttempt(ctx context.Context, attemptID int64, callback Callback) (AttemptResult, error) {
	if attemptID <= 0 {
		return "", newProcessingError(FailureTransaction, ErrInvalidAttempt)
	}
	snapshot, err := s.verify(ctx, callback)
	if err != nil {
		return "", err
	}
	result, err := s.Repo.ProcessAttempt(ctx, attemptID, callback)
	return result, enrichProcessingError(err, true, snapshot.OrderID)
}

func (s *Service) verify(ctx context.Context, callback Callback) (VerificationSnapshot, error) {
	if s == nil || s.Repo == nil {
		return VerificationSnapshot{}, newProcessingError(FailureDependency, readerDependencyUnavailable)
	}
	if strings.TrimSpace(callback.OrderNo) == "" {
		return VerificationSnapshot{}, newProcessingError(FailureUnknownOrder, ErrUnauthorized)
	}
	snapshot, err := s.Repo.VerificationSnapshot(ctx, callback.OrderNo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VerificationSnapshot{}, newProcessingError(FailureUnknownOrder, ErrUnauthorized)
		}
		return VerificationSnapshot{}, newProcessingError(FailureDependency, readerDependencyUnavailable)
	}
	credentials := s.Credentials
	if s.LoadCredentials != nil {
		credentials, err = s.LoadCredentials(ctx)
		if err != nil {
			return VerificationSnapshot{}, enrichProcessingError(newProcessingError(FailureDependency, readerDependencyUnavailable), false, snapshot.OrderID)
		}
	}
	if credentials == nil {
		return VerificationSnapshot{}, enrichProcessingError(newProcessingError(FailureUnknownCredential, ErrUnauthorized), false, snapshot.OrderID)
	}
	credential, ok := credentials.Verification(snapshot.CredentialRef)
	if !ok {
		return VerificationSnapshot{}, enrichProcessingError(newProcessingError(FailureUnknownCredential, ErrUnauthorized), false, snapshot.OrderID)
	}
	expectedPID := strings.TrimSpace(snapshot.MerchantPID)
	if expectedPID == "" {
		if strings.TrimSpace(snapshot.CredentialRef) != "" {
			return VerificationSnapshot{}, enrichProcessingError(newProcessingError(FailurePIDMismatch, ErrUnauthorized), false, snapshot.OrderID)
		}
		expectedPID = credential.PID()
	}
	if credential.PID() != expectedPID || callback.PID != expectedPID || callback.Fields["pid"] != callback.PID {
		return VerificationSnapshot{}, enrichProcessingError(newProcessingError(FailurePIDMismatch, ErrUnauthorized), false, snapshot.OrderID)
	}
	if callback.Fields["order_id"] != callback.OrderNo {
		return VerificationSnapshot{}, enrichProcessingError(newProcessingError(FailureSnapshotMismatch, ErrRejected), false, snapshot.OrderID)
	}
	if callback.Fields["signature"] != callback.Signature || !Verify(callback.Fields, callback.Signature, credential.Secret()) {
		return VerificationSnapshot{}, enrichProcessingError(newProcessingError(FailureSignatureInvalid, ErrUnauthorized), false, snapshot.OrderID)
	}
	return snapshot, nil
}

var readerDependencyUnavailable = errors.New("payment callback dependency unavailable")
