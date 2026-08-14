-- +goose Up
-- +goose StatementBegin
ALTER TABLE commerce_membership_grants ADD COLUMN IF NOT EXISTS remark varchar(255) NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS commerce_membership_grants_admin_request_uidx ON commerce_membership_grants (reader_id, source_type, source_ref) WHERE grant_type='admin' AND source_type='admin' AND source_ref <> '';
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1738,now(),now(),'/reader/users/:id/membership','人工发放会员','读者运营','POST') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/users/:id/membership','POST','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1738),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
