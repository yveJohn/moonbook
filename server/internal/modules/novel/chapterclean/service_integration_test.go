package chapterclean

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestConfigLifecyclePostgreSQL(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_CHAPTER_CLEAN_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_CHAPTER_CLEAN_TEST_DSN is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var configID, modelID int64
	err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config(config_name,base_url,stream_mode,failure_threshold,enabled) VALUES('clean-test','http://127.0.0.1:1','NON_STREAM',2,true) RETURNING id`).Scan(&configID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config_model(ai_config_id,model_name,sort_order) VALUES($1,'test-model',1) RETURNING id`, configID).Scan(&modelID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2 WHERE id=$1`, configID, modelID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, nil)
	input := ConfigInput{AIConfigID: stringID(configID), Enabled: true, SystemPrompt: "clean", Temperature: .1, MinCleanedTextPercent: 70, AutoSuccessWordCount: 3000, TimeoutSeconds: 30, RetryCount: 1}
	first, err := service.SaveConfig(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.RequestIntervalMS = 25
	second, err := service.SaveConfig(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || second.RequestIntervalMS != 25 {
		t.Fatalf("singleton update failed: first=%+v second=%+v", first, second)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapter_clean_config`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func stringID(id int64) string { return fmt.Sprintf("%d", id) }
