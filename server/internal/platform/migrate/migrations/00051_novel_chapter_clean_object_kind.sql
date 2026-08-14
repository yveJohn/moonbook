-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_objects DROP CONSTRAINT novel_objects_kind_check;
ALTER TABLE novel_objects ADD CONSTRAINT novel_objects_kind_check
    CHECK (object_kind IN ('chapter_content', 'chapter_clean', 'book_cover'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
