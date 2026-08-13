-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_objects
    ADD COLUMN source_fingerprint char(64),
    ADD CONSTRAINT novel_objects_source_fingerprint_check
        CHECK (source_fingerprint IS NULL OR source_fingerprint ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT novel_objects_legacy_fingerprint_check
        CHECK (source <> 'legacy' OR source_fingerprint IS NOT NULL);

CREATE UNIQUE INDEX novel_objects_source_fingerprint_unique
    ON novel_objects (object_kind, book_id, owner_id, source_fingerprint)
    WHERE source_fingerprint IS NOT NULL AND state IN ('verified', 'active');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration';
END
$$;
-- +goose StatementEnd
