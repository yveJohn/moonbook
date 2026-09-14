package serverchan

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/secretcrypto"
)

var (
	ErrConfiguration = errors.New("Server酱配置不可用")
	ErrRequest       = errors.New("Server酱请求失败")
	ErrResponse      = errors.New("Server酱响应失败")
)

type Config struct {
	Enabled bool
	SendKey string
}

type ConfigLoader interface {
	Load(context.Context) (Config, error)
}

type SQLConfigLoader struct {
	DB     *sql.DB
	Cipher *secretcrypto.Cipher
}

func (loader SQLConfigLoader) Load(ctx context.Context) (Config, error) {
	if loader.DB == nil {
		return Config{}, ErrConfiguration
	}
	var enabled string
	err := loader.DB.QueryRowContext(ctx, `SELECT value FROM sys_params WHERE key=$1 AND deleted_at IS NULL ORDER BY id LIMIT 1`, EnabledParamKey).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, ErrConfiguration
	}
	if strings.TrimSpace(enabled) != "true" {
		return Config{}, nil
	}

	var ciphertext string
	err = loader.DB.QueryRowContext(ctx, `SELECT value FROM sys_params WHERE key=$1 AND deleted_at IS NULL ORDER BY id LIMIT 1`, SendKeyParamKey).Scan(&ciphertext)
	if err != nil || loader.Cipher == nil || strings.TrimSpace(ciphertext) == "" {
		return Config{}, ErrConfiguration
	}
	sendKey, err := loader.Cipher.Decrypt(ciphertext, SendKeyScope())
	if err != nil || strings.TrimSpace(sendKey) == "" {
		return Config{}, ErrConfiguration
	}
	return Config{Enabled: true, SendKey: strings.TrimSpace(sendKey)}, nil
}
