package aiconfig

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestAIConfigLifecycleAndRuntimeModelSwitchWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_AI_CONFIG_TEST_DSN")
	if dsn == "" {
		t.Skip("Moonbook AI config integration environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	secretName := "MOONBOOK_AI_INTEGRATION_API_KEY"
	service := NewService(db, func(name string) (string, bool) {
		if name == secretName {
			return "integration-only-secret", true
		}
		return "", false
	})
	created, err := service.Create(ctx, Input{
		ConfigName: "集成 AI " + strconv.FormatInt(time.Now().UnixNano(), 10), BaseURL: "http://127.0.0.1:18080/v1",
		StreamMode: "AUTO", Models: []ModelInput{{ModelName: "model-a"}, {ModelName: "model-b"}},
		FailureThreshold: "2", SecretEnvName: secretName, Enabled: true, Remark: "integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.ParseInt(created.ID, 10, 64)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `UPDATE novel_ai_config SET current_model_id=NULL WHERE id=$1`, id)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM novel_ai_config_model WHERE ai_config_id=$1`, id)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM novel_ai_config WHERE id=$1`, id)
	})
	if created.ID == "" || len(created.Models) != 2 || created.CurrentModelName != "model-a" || !created.SecretConfigured || created.StateVersion != "0" {
		t.Fatalf("created=%+v", created)
	}
	page, err := service.List(ctx, "model-b", "", 1, 20)
	if err != nil || page.Total < 1 {
		t.Fatalf("unfiltered enabled state list=%+v err=%v", page, err)
	}
	runtime, err := service.ResolveEnabled(ctx, created.ID)
	if err != nil || runtime.APIKey() != "integration-only-secret" || runtime.ModelName != "model-a" {
		t.Fatalf("runtime=%+v err=%v", runtime, err)
	}
	if err := service.RecordCallFailure(ctx, runtime.Snapshot()); err != nil {
		t.Fatal(err)
	}
	afterOne, err := service.Get(ctx, created.ID)
	if err != nil || afterOne.ConsecutiveFailures != 1 || afterOne.CurrentModelName != "model-a" {
		t.Fatalf("after one=%+v err=%v", afterOne, err)
	}
	if err := service.RecordCallFailure(ctx, runtime.Snapshot()); err != nil {
		t.Fatal(err)
	}
	afterSwitch, err := service.Get(ctx, created.ID)
	if err != nil || afterSwitch.ConsecutiveFailures != 0 || afterSwitch.CurrentModelName != "model-b" || afterSwitch.StateVersion != "1" {
		t.Fatalf("after switch=%+v err=%v", afterSwitch, err)
	}
	if err := service.RecordCallFailure(ctx, runtime.Snapshot()); err != nil {
		t.Fatal(err)
	}
	staleIgnored, _ := service.Get(ctx, created.ID)
	if staleIgnored.ConsecutiveFailures != 0 || staleIgnored.CurrentModelName != "model-b" {
		t.Fatalf("stale snapshot changed state: %+v", staleIgnored)
	}
	reset, err := service.ResetModelState(ctx, created.ID)
	if err != nil || reset.CurrentModelName != "model-a" || reset.StateVersion != "2" {
		t.Fatalf("reset=%+v err=%v", reset, err)
	}
	updated, err := service.Update(ctx, created.ID, Input{
		ConfigName: created.ConfigName, BaseURL: created.BaseURL, StreamMode: "NON_STREAM",
		Models: []ModelInput{{ModelName: "model-c"}, {ModelName: "model-a"}}, FailureThreshold: "3",
		SecretEnvName: secretName, Enabled: true, Remark: "updated",
	})
	if err != nil || updated.CurrentModelName != "model-c" || updated.StateVersion != "3" || updated.FailureThreshold != 3 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	options, err := service.EnabledOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, option := range options {
		if option.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("enabled options did not include created config")
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveEnabled(ctx, created.ID); err == nil {
		t.Fatal("deleted config must not resolve at runtime")
	}
}
