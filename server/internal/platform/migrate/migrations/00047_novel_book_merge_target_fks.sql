-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_book_merge_task
    ADD CONSTRAINT novel_book_merge_task_target_book_fk
    FOREIGN KEY(target_book_id) REFERENCES novel_books(id);

ALTER TABLE novel_book_merge_chapter
    ADD CONSTRAINT novel_book_merge_chapter_target_book_fk
    FOREIGN KEY(target_book_id) REFERENCES novel_books(id);

ALTER TABLE novel_book_merge_chapter
    ADD CONSTRAINT novel_book_merge_chapter_target_chapter_fk
    FOREIGN KEY(target_chapter_id) REFERENCES novel_chapters(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
