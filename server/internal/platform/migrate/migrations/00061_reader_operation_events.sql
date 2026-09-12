CREATE TABLE reader_operation_events (
    id BIGSERIAL PRIMARY KEY,
    reader_id BIGINT NOT NULL REFERENCES reader_accounts(id),
    event_type VARCHAR(64) NOT NULL,
    event_name VARCHAR(128) NOT NULL,
    target_id BIGINT,
    detail VARCHAR(500) NOT NULL DEFAULT '',
    ip VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX reader_operation_events_reader_created_idx ON reader_operation_events(reader_id, created_at DESC, id DESC);
