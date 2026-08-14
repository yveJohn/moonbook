-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1710,now(),now(),2,1700,'rechargeSettings','ReaderRechargeSettings',false,'view/reader/payment/rechargeSettings/index.vue',2,'',true,false,'自定义充值规则','setting',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1710,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES
(1726,now(),now(),'/reader/payment/rechargeSettings','查询自定义充值规则','读者支付','GET'),
(1727,now(),now(),'/reader/payment/rechargeSettings','保存自定义充值规则','读者支付','PUT') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES
('p','888','/reader/payment/rechargeSettings','GET','','',''),('p','888','/reader/payment/rechargeSettings','PUT','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1710),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1727),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
