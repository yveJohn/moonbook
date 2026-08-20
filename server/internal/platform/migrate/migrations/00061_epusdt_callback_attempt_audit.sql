-- +goose Up
-- +goose StatementBegin
ALTER TABLE reader_payment_callback_logs
    ADD COLUMN failure_code varchar(64),
    ADD COLUMN request_id varchar(64),
    ADD COLUMN trace_id varchar(64),
    ADD COLUMN payload_bytes integer,
    ADD COLUMN payload_truncated boolean,
    ADD COLUMN completed_at timestamptz,
    ADD CONSTRAINT reader_payment_callback_logs_payload_bytes_check
        CHECK (payload_bytes IS NULL OR payload_bytes BETWEEN 0 AND 16385),
    ADD CONSTRAINT reader_payment_callback_logs_failure_code_check
        CHECK (failure_code IS NULL OR failure_code ~ '^[A-Z][A-Z0-9_]{0,63}$');

CREATE INDEX reader_payment_callback_logs_result_created_idx
    ON reader_payment_callback_logs (processing_result, created_at DESC, id DESC);

CREATE INDEX reader_payment_callback_logs_failure_created_idx
    ON reader_payment_callback_logs (failure_code, created_at DESC, id DESC)
    WHERE failure_code IS NOT NULL;

CREATE INDEX reader_payment_callback_logs_response_created_idx
    ON reader_payment_callback_logs (response_status, created_at DESC, id DESC);

CREATE INDEX reader_payment_callback_logs_request_time_idx
    ON reader_payment_callback_logs (request_time DESC, id DESC);

CREATE INDEX reader_payment_callback_logs_runtime_received_idx
    ON reader_payment_callback_logs (created_at, id)
    WHERE source_type = 'runtime' AND processing_result = 'received';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
