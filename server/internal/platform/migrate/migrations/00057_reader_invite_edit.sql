-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES
    (1809,now(),now(),'/reader/inviteCodes/:id','编辑邀请码','读者运营','PUT')
ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES
    ('p','888','/reader/inviteCodes/:id','PUT','','','')
ON CONFLICT DO NOTHING;
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1809),true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
