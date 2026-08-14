-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1712,now(),now(),2,1000,'readerOrders','ReaderOrders',false,'view/reader/orders/index.vue',36,'',true,false,'消费订单','tickets',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1712,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1736,now(),now(),'/reader/orders','查询消费订单','读者运营','GET'),(1737,now(),now(),'/reader/orders/:id','查询消费订单详情','读者运营','GET') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/orders','GET','','',''),('p','888','/reader/orders/:id','GET','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1712),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1737),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
