-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_txt_import_task
    ALTER COLUMN object_key DROP NOT NULL,
    ALTER COLUMN object_sha256 DROP NOT NULL,
    ALTER COLUMN object_byte_size DROP NOT NULL;
DROP INDEX IF EXISTS novel_txt_import_object_unique;
CREATE UNIQUE INDEX novel_txt_import_object_unique
    ON novel_txt_import_task(target_book_id, object_sha256)
    WHERE object_sha256 IS NOT NULL;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
