-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_crawl_forum_source ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_crawl_forum_board ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_crawl_import_task ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_crawl_thread_candidate ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_crawl_fetch_log ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_txt_import_task ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_chapter_clean_task ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_chapter_clean_result ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_book_merge_task ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_book_merge_source ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_book_merge_chapter ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_ai_config ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_ai_config_model ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);

CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_crawl_forum_source_legacy_key ON novel_crawl_forum_source(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_crawl_forum_board_legacy_key ON novel_crawl_forum_board(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_crawl_import_task_legacy_key ON novel_crawl_import_task(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_crawl_thread_candidate_legacy_key ON novel_crawl_thread_candidate(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_crawl_fetch_log_legacy_key ON novel_crawl_fetch_log(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_txt_import_task_legacy_key ON novel_txt_import_task(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_chapter_clean_task_legacy_key ON novel_chapter_clean_task(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_chapter_clean_result_legacy_key ON novel_chapter_clean_result(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_book_merge_task_legacy_key ON novel_book_merge_task(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_book_merge_source_legacy_key ON novel_book_merge_source(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_book_merge_chapter_legacy_key ON novel_book_merge_chapter(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_ai_config_legacy_key ON novel_ai_config(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_ai_config_model_legacy_key ON novel_ai_config_model(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
