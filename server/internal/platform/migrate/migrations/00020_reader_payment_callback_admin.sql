-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1704,now(),now(),2,1700,'callbackLogs','ReaderPaymentCallbackLogs',false,'view/reader/payment/callbackLogs/index.vue',4,'',true,false,'回调日志','document',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1704,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1708,now(),now(),'/reader/payment/callbackLogs','查询支付回调日志','读者支付','GET'),(1709,now(),now(),'/reader/payment/callbackLogs/:id','查询支付回调日志详情','读者支付','GET') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/payment/callbackLogs','GET','','',''),('p','888','/reader/payment/callbackLogs/:id','GET','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1704),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1709),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
