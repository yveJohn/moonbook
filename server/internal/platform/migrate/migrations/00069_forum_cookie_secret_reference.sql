-- +goose Up
-- +goose StatementBegin
ALTER TABLE novel_crawl_forum_source
    ADD COLUMN cookie_secret_ref varchar(128) NOT NULL DEFAULT '',
    ADD CONSTRAINT novel_crawl_forum_source_cookie_secret_ref_check
        CHECK (cookie_secret_ref = '' OR cookie_secret_ref ~ '^MOONBOOK_FORUM_COOKIE_[A-Z0-9_]+$');

CREATE TABLE novel_crawl_cookie_secret_migration_audit (
    source_id_hash char(64) PRIMARY KEY,
    had_legacy_cookie boolean NOT NULL,
    disposition varchar(32) NOT NULL,
    migrated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT novel_crawl_cookie_secret_audit_disposition_check
        CHECK (disposition IN ('secret_reference_required', 'not_applicable'))
);

INSERT INTO novel_crawl_cookie_secret_migration_audit(source_id_hash, had_legacy_cookie, disposition)
SELECT encode(sha256(convert_to(id::text, 'UTF8')), 'hex'),
       cookie_text IS NOT NULL AND cookie_text <> '',
       CASE WHEN cookie_text IS NOT NULL AND cookie_text <> ''
            THEN 'secret_reference_required'
            ELSE 'not_applicable'
       END
FROM novel_crawl_forum_source;

UPDATE novel_crawl_forum_source
SET cookie_text = NULL
WHERE cookie_text IS NOT NULL;

ALTER TABLE novel_crawl_forum_source
    ADD CONSTRAINT novel_crawl_forum_source_cookie_text_cleared_check CHECK (cookie_text IS NULL);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
