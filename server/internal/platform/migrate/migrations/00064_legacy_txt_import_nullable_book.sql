-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_txt_import_task ALTER COLUMN target_book_id DROP NOT NULL;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
