package secretcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const EnvMasterKey = "MOONBOOK_APP_MASTER_KEY"

var (
	ErrInvalidMasterKey  = errors.New("application master key is invalid")
	ErrInvalidScope      = errors.New("secret encryption scope is invalid")
	ErrInvalidCiphertext = errors.New("encrypted application secret is invalid")
)

type LookupEnv func(string) (string, bool)

type Scope struct {
	Table    string `json:"table"`
	RecordID string `json:"recordId"`
	Field    string `json:"field"`
}

type Cipher struct {
	aead cipher.AEAD
}

func NewFromEnv(lookup LookupEnv) (*Cipher, error) {
	if lookup == nil {
		return nil, ErrInvalidMasterKey
	}
	value, _ := lookup(EnvMasterKey)
	return New(value)
}

func New(encodedKey string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidMasterKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidMasterKey
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalidMasterKey
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext string, scope Scope) (string, error) {
	if c == nil || c.aead == nil {
		return "", ErrInvalidMasterKey
	}
	aad, err := scopeAAD(scope)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", ErrInvalidCiphertext
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), aad)
	envelope := append(nonce, sealed...)
	return "v1:" + base64.StdEncoding.EncodeToString(envelope), nil
}

func (c *Cipher) Decrypt(envelope string, scope Scope) (string, error) {
	if c == nil || c.aead == nil {
		return "", ErrInvalidMasterKey
	}
	aad, err := scopeAAD(scope)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(envelope, "v1:") {
		return "", ErrInvalidCiphertext
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(envelope, "v1:"))
	if err != nil || len(raw) < c.aead.NonceSize()+c.aead.Overhead() {
		return "", ErrInvalidCiphertext
	}
	nonce, ciphertext := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plaintext), nil
}

func scopeAAD(scope Scope) ([]byte, error) {
	if strings.TrimSpace(scope.Table) == "" || strings.TrimSpace(scope.RecordID) == "" || strings.TrimSpace(scope.Field) == "" {
		return nil, ErrInvalidScope
	}
	value, err := json.Marshal(struct {
		Version string `json:"version"`
		Scope
	}{Version: "v1", Scope: scope})
	if err != nil {
		return nil, ErrInvalidScope
	}
	return value, nil
}
