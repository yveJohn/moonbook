-- +goose Up
-- +goose StatementBegin
-- The write endpoint is removed from the router; remove stale permissions so
-- administrators cannot mistake the historical GVA seed for a live capability.
DELETE FROM casbin_rule
 WHERE ptype = 'p' AND v1 = '/system/setSystemConfig' AND v2 = 'POST';
DELETE FROM sys_apis
 WHERE path = '/system/setSystemConfig' AND method = 'POST';
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
