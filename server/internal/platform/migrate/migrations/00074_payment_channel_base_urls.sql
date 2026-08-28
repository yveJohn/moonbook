-- +goose Up
-- +goose StatementBegin
ALTER TABLE reader_payment_channels
    ADD COLUMN epusdt_base_url varchar(2048) NOT NULL DEFAULT '',
    ADD COLUMN reader_base_url varchar(2048) NOT NULL DEFAULT '';

ALTER TABLE reader_payment_channels
    DROP CONSTRAINT reader_payment_channels_enabled_config_check,
    ADD CONSTRAINT reader_payment_channels_enabled_config_check CHECK (
        NOT enabled OR (
            archived_at IS NULL
            AND merchant_pid_ciphertext <> ''
            AND secret_ciphertext <> ''
            AND epusdt_base_url <> ''
            AND reader_base_url <> ''
        )
    ) NOT VALID;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
