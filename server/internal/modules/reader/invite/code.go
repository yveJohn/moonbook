package invite

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	inviteCodeAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	inviteCodeLength   = 8
	inviteCodeAttempts = 8
)

// GenerateRandomCode returns an 8-character uppercase alphanumeric invite code.
func GenerateRandomCode() (string, error) {
	out := make([]byte, inviteCodeLength)
	max := big.NewInt(int64(len(inviteCodeAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = inviteCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

func generatedCode() (string, error) {
	return GenerateRandomCode()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
