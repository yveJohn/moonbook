-- +goose Up
-- +goose StatementBegin
CREATE TABLE commerce_reader_search_projection (
    reader_id bigint PRIMARY KEY,
    username varchar(64) NOT NULL,
    nickname varchar(64) NOT NULL DEFAULT '',
    status varchar(16) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT commerce_reader_search_projection_username_not_blank CHECK (btrim(username) <> ''),
    CONSTRAINT commerce_reader_search_projection_status_check CHECK (status IN ('enabled', 'disabled', 'deleted'))
);

CREATE INDEX commerce_reader_search_projection_username_idx
    ON commerce_reader_search_projection (lower(username), reader_id);
CREATE INDEX commerce_reader_search_projection_nickname_idx
    ON commerce_reader_search_projection (lower(nickname), reader_id);
CREATE INDEX commerce_reader_search_projection_status_idx
    ON commerce_reader_search_projection (status, reader_id);

INSERT INTO commerce_reader_search_projection (
    reader_id,
    username,
    nickname,
    status,
    created_at,
    updated_at
)
SELECT
    id,
    username,
    nickname,
    status,
    created_at,
    updated_at
FROM reader_accounts
ON CONFLICT (reader_id) DO UPDATE
SET username = EXCLUDED.username,
    nickname = EXCLUDED.nickname,
    status = EXCLUDED.status,
    created_at = EXCLUDED.created_at,
    updated_at = EXCLUDED.updated_at;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
