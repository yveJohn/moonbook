-- +goose Up
-- +goose StatementBegin
ALTER TABLE reader_recharge_orders
    ADD COLUMN credential_ref varchar(100),
    ADD COLUMN merchant_pid_snapshot varchar(255),
    ADD COLUMN active_reader_id bigint REFERENCES reader_accounts(id);

DO $$
BEGIN
    IF EXISTS (
        SELECT gateway_trade_id
        FROM reader_recharge_orders
        WHERE gateway_trade_id IS NOT NULL AND btrim(gateway_trade_id) <> ''
        GROUP BY gateway_trade_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'reader_recharge_orders contains duplicate non-empty gateway_trade_id values';
    END IF;

    IF EXISTS (
        SELECT block_transaction_id
        FROM reader_recharge_orders
        WHERE block_transaction_id IS NOT NULL AND btrim(block_transaction_id) <> ''
        GROUP BY block_transaction_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'reader_recharge_orders contains duplicate non-empty block_transaction_id values';
    END IF;
END $$;

WITH ranked_active_orders AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY reader_id
            ORDER BY created_at DESC, id DESC
        ) AS activity_rank
    FROM reader_recharge_orders
    WHERE status IN ('creating', 'pending', 'gateway_unknown')
)
UPDATE reader_recharge_orders AS orders
SET status = 'superseded',
    active_reader_id = NULL,
    failure_code = 'ORDER_REPLACED',
    failure_message = 'Replaced by the latest active order during migration',
    updated_at = now()
FROM ranked_active_orders AS ranked
WHERE orders.id = ranked.id
  AND ranked.activity_rank > 1;

UPDATE reader_recharge_orders
SET active_reader_id = reader_id
WHERE status IN ('creating', 'pending', 'gateway_unknown');

CREATE UNIQUE INDEX reader_recharge_orders_gateway_trade_id_uidx
    ON reader_recharge_orders (gateway_trade_id)
    WHERE gateway_trade_id IS NOT NULL AND btrim(gateway_trade_id) <> '';

CREATE UNIQUE INDEX reader_recharge_orders_block_transaction_id_uidx
    ON reader_recharge_orders (block_transaction_id)
    WHERE block_transaction_id IS NOT NULL AND btrim(block_transaction_id) <> '';

CREATE UNIQUE INDEX reader_recharge_orders_active_reader_id_uidx
    ON reader_recharge_orders (active_reader_id)
    WHERE active_reader_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
