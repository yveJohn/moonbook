package serverchan

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/secretcrypto"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

func setupConfigLoader(t *testing.T) (SQLConfigLoader, *secretcrypto.Cipher) {
	t.Helper()
	db := testutil.NewMemoryDBWithoutGlobal(t, &system.SysParams{})
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := secretcrypto.New(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	return SQLConfigLoader{DB: sqlDB, Cipher: cipher}, cipher
}

func TestSQLConfigLoaderTreatsMissingDisabledAndInvalidSwitchAsDisabled(t *testing.T) {
	loader, _ := setupConfigLoader(t)
	ctx := context.Background()
	config, err := loader.Load(ctx)
	if err != nil || config.Enabled {
		t.Fatalf("missing config=%+v err=%v", config, err)
	}
	for _, value := range []string{"false", "invalid"} {
		if _, err := loader.DB.ExecContext(ctx, `DELETE FROM sys_params`); err != nil {
			t.Fatal(err)
		}
		if _, err := loader.DB.ExecContext(ctx, `INSERT INTO sys_params(name,key,value,created_at,updated_at) VALUES('开关',$1,$2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, EnabledParamKey, value); err != nil {
			t.Fatal(err)
		}
		config, err = loader.Load(ctx)
		if err != nil || config.Enabled {
			t.Fatalf("value=%q config=%+v err=%v", value, config, err)
		}
	}
}

func TestSQLConfigLoaderDecryptsEnabledSendKey(t *testing.T) {
	loader, cipher := setupConfigLoader(t)
	ctx := context.Background()
	ciphertext, err := cipher.Encrypt("SCT_test_secret_123456", SendKeyScope())
	if err != nil {
		t.Fatal(err)
	}
	for _, param := range []struct{ key, value string }{{EnabledParamKey, "true"}, {SendKeyParamKey, ciphertext}} {
		if _, err = loader.DB.ExecContext(ctx, `INSERT INTO sys_params(name,key,value,created_at,updated_at) VALUES('参数',$1,$2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, param.key, param.value); err != nil {
			t.Fatal(err)
		}
	}
	config, err := loader.Load(ctx)
	if err != nil || !config.Enabled || config.SendKey != "SCT_test_secret_123456" {
		t.Fatalf("config=%+v err=%v", config, err)
	}

	if _, err = loader.DB.ExecContext(ctx, `UPDATE sys_params SET value='broken' WHERE key=$1`, SendKeyParamKey); err != nil {
		t.Fatal(err)
	}
	if _, err = loader.Load(ctx); err != ErrConfiguration {
		t.Fatalf("broken ciphertext err=%v", err)
	}
}
