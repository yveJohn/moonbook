-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_objects
    ADD CONSTRAINT novel_objects_target_id_unique UNIQUE (object_kind, book_id, owner_id, id);

ALTER TABLE novel_object_references
    ADD CONSTRAINT novel_object_references_target_fk
    FOREIGN KEY (object_kind, book_id, owner_id, object_id)
    REFERENCES novel_objects (object_kind, book_id, owner_id, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration';
END
$$;
-- +goose StatementEnd
