-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES
(1826,now(),now(),'/reader/users/:id/operations','查询读者后台操作日志','读者运营','GET') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES
('p','888','/reader/users/:id/operations','GET','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1826),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
