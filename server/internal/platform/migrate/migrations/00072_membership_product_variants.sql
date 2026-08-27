-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS commerce_products_target_unique;
CREATE UNIQUE INDEX commerce_products_target_unique
    ON commerce_products(product_type, target_id)
    WHERE target_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
