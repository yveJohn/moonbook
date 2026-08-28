-- +goose Up
-- +goose StatementBegin
UPDATE reader_payment_channels
SET enabled = false,
    updated_at = now()
WHERE enabled
  AND (epusdt_base_url = '' OR reader_base_url = '');

ALTER TABLE reader_payment_channels
    VALIDATE CONSTRAINT reader_payment_channels_enabled_config_check;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
