-- +goose Up
-- +goose StatementBegin
DELETE FROM casbin_rule
WHERE ptype='p' AND v1 IN ('/email/emailTest','/email/sendEmail') AND v2='POST';

DELETE FROM sys_apis
WHERE path IN ('/email/emailTest','/email/sendEmail') AND method='POST';
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
