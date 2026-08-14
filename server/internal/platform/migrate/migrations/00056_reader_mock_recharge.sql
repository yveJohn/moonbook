-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES
    (1807,now(),now(),'/reader/orders/mockRecharge','创建模拟充值订单','读者运营','POST'),
    (1808,now(),now(),'/reader/orders/:id/confirmMockRecharge','确认模拟充值订单','读者运营','POST')
ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES
    ('p','888','/reader/orders/mockRecharge','POST','','',''),
    ('p','888','/reader/orders/:id/confirmMockRecharge','POST','','','')
ON CONFLICT DO NOTHING;
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1808),true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
