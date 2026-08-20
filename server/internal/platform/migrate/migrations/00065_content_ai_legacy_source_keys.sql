-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_chapter_summary_task ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_book_profile_config ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);
ALTER TABLE novel_book_profile_suggestion ADD COLUMN IF NOT EXISTS legacy_source_key varchar(191);

CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_chapter_summary_task_legacy_key ON novel_chapter_summary_task(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_book_profile_config_legacy_key ON novel_book_profile_config(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_novel_book_profile_suggestion_legacy_key ON novel_book_profile_suggestion(legacy_source_key) WHERE legacy_source_key IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
