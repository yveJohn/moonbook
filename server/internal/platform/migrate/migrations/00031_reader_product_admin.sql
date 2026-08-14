-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS commerce_products_id_seq;
SELECT setval('commerce_products_id_seq', GREATEST(COALESCE((SELECT max(id) FROM commerce_products),2607300200000),2607300200000), true);
ALTER TABLE commerce_products ALTER COLUMN id SET DEFAULT nextval('commerce_products_id_seq');
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1711,now(),now(),2,1000,'readerProducts','ReaderProducts',false,'view/reader/products/index.vue',35,'',true,false,'消费商品','shopping-bag',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1711,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1732,now(),now(),'/reader/products','查询消费商品','读者运营','GET'),(1733,now(),now(),'/reader/products','新增消费商品','读者运营','POST'),(1734,now(),now(),'/reader/products/:id','修改消费商品','读者运营','PUT'),(1735,now(),now(),'/reader/products/:id','删除消费商品','读者运营','DELETE') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/products','GET','','',''),('p','888','/reader/products','POST','','',''),('p','888','/reader/products/:id','PUT','','',''),('p','888','/reader/products/:id','DELETE','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1711),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1735),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
