package system

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/secretcrypto"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/serverchan"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
)

func setupSysParamsService(t *testing.T) (*SysParamsService, string) {
	t.Helper()
	testutil.NewMemoryDB(t, &model.SysParams{})
	masterKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv(secretcrypto.EnvMasterKey, masterKey)
	return &SysParamsService{}, masterKey
}

func TestSysParamsProtectsServerChanSendKey(t *testing.T) {
	service, masterKey := setupSysParamsService(t)
	ctx := context.Background()
	param := model.SysParams{Name: "Server酱 SendKey", Key: serverchan.SendKeyParamKey, Value: "SCT_test_secret_123456", Desc: "反馈通知"}
	if err := service.CreateSysParams(ctx, &param); err != nil {
		t.Fatal(err)
	}

	var stored model.SysParams
	if err := global.GVA_DB.WithContext(ctx).First(&stored, param.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Value == "SCT_test_secret_123456" {
		t.Fatalf("SendKey was not encrypted: %q", stored.Value)
	}
	cipher, err := secretcrypto.New(masterKey)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := cipher.Decrypt(stored.Value, serverchan.SendKeyScope())
	if err != nil || plaintext != "SCT_test_secret_123456" {
		t.Fatalf("plaintext=%q err=%v", plaintext, err)
	}

	byID, err := service.GetSysParams(ctx, strconv.FormatUint(uint64(param.ID), 10))
	if err != nil || byID.Value != serverchan.SecretMask {
		t.Fatalf("by ID=%+v err=%v", byID, err)
	}
	byKey, err := service.GetSysParam(ctx, serverchan.SendKeyParamKey)
	if err != nil || byKey.Value != serverchan.SecretMask {
		t.Fatalf("by key=%+v err=%v", byKey, err)
	}
	list, _, err := service.GetSysParamsInfoList(ctx, systemReq.SysParamsSearch{})
	if err != nil || len(list) != 1 || list[0].Value != serverchan.SecretMask {
		t.Fatalf("list=%+v err=%v", list, err)
	}
}

func TestSysParamsServerChanSendKeyUpdatePreservesOrRotatesSecret(t *testing.T) {
	service, masterKey := setupSysParamsService(t)
	ctx := context.Background()
	param := model.SysParams{Name: "Server酱 SendKey", Key: serverchan.SendKeyParamKey, Value: "SCT_old_secret_123456"}
	if err := service.CreateSysParams(ctx, &param); err != nil {
		t.Fatal(err)
	}
	var before model.SysParams
	if err := global.GVA_DB.WithContext(ctx).First(&before, param.ID).Error; err != nil {
		t.Fatal(err)
	}

	param.Value = serverchan.SecretMask
	param.Desc = "保留原值"
	if err := service.UpdateSysParams(ctx, param); err != nil {
		t.Fatal(err)
	}
	var preserved model.SysParams
	if err := global.GVA_DB.WithContext(ctx).First(&preserved, param.ID).Error; err != nil {
		t.Fatal(err)
	}
	if preserved.Value != before.Value {
		t.Fatal("masked update replaced encrypted SendKey")
	}

	param.Value = "SCT_new_secret_654321"
	if err := service.UpdateSysParams(ctx, param); err != nil {
		t.Fatal(err)
	}
	var rotated model.SysParams
	if err := global.GVA_DB.WithContext(ctx).First(&rotated, param.ID).Error; err != nil {
		t.Fatal(err)
	}
	cipher, _ := secretcrypto.New(masterKey)
	plaintext, err := cipher.Decrypt(rotated.Value, serverchan.SendKeyScope())
	if err != nil || plaintext != "SCT_new_secret_654321" || rotated.Value == preserved.Value {
		t.Fatalf("plaintext=%q rotated=%t err=%v", plaintext, rotated.Value != preserved.Value, err)
	}
}

func TestSysParamsValidatesServerChanReservedParams(t *testing.T) {
	service, _ := setupSysParamsService(t)
	ctx := context.Background()
	enabled := model.SysParams{Name: "Server酱开关", Key: serverchan.EnabledParamKey, Value: "yes"}
	if err := service.CreateSysParams(ctx, &enabled); !errors.Is(err, ErrInvalidServerChanValue) {
		t.Fatalf("invalid enabled err=%v", err)
	}
	enabled.Value = " false "
	if err := service.CreateSysParams(ctx, &enabled); err != nil || enabled.Value != "false" {
		t.Fatalf("enabled=%q err=%v", enabled.Value, err)
	}
	duplicate := model.SysParams{Name: "重复开关", Key: serverchan.EnabledParamKey, Value: "true"}
	if err := service.CreateSysParams(ctx, &duplicate); !errors.Is(err, ErrReservedParamDuplicate) {
		t.Fatalf("duplicate err=%v", err)
	}

	enabled.Key = "serverchan.renamed"
	if err := service.UpdateSysParams(ctx, enabled); !errors.Is(err, ErrReservedParamRename) {
		t.Fatalf("reserved rename err=%v", err)
	}
	ordinary := model.SysParams{Name: "普通参数", Key: "ordinary.key", Value: "value"}
	if err := service.CreateSysParams(ctx, &ordinary); err != nil {
		t.Fatal(err)
	}
	ordinary.Key = serverchan.SendKeyParamKey
	ordinary.Value = "SCT_secret_123456789"
	if err := service.UpdateSysParams(ctx, ordinary); !errors.Is(err, ErrReservedParamRename) {
		t.Fatalf("rename into reserved err=%v", err)
	}
}
