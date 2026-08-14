-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS reader_recharge_products_id_seq;
SELECT setval('reader_recharge_products_id_seq', GREATEST(COALESCE((SELECT max(id) FROM reader_recharge_products),2607300100000),2607300100000), true);
ALTER TABLE reader_recharge_products ALTER COLUMN id SET DEFAULT nextval('reader_recharge_products_id_seq');
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1700,now(),now(),1,1000,'readerPayment','ReaderPayment',false,'',20,'',true,false,'读者支付', 'money',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1701,now(),now(),2,1700,'rechargeProducts','ReaderRechargeProducts',false,'view/reader/payment/rechargeProducts/index.vue',1,'',true,false,'充值档位','shopping-cart',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1700,888),(1701,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1700,now(),now(),'/reader/payment/rechargeProducts','查询充值档位','读者支付','GET'),(1701,now(),now(),'/reader/payment/rechargeProducts','新增充值档位','读者支付','POST'),(1702,now(),now(),'/reader/payment/rechargeProducts/:id','修改充值档位','读者支付','PUT'),(1703,now(),now(),'/reader/payment/rechargeProducts/:id','删除充值档位','读者支付','DELETE') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/payment/rechargeProducts','GET','','',''),('p','888','/reader/payment/rechargeProducts','POST','','',''),('p','888','/reader/payment/rechargeProducts/:id','PUT','','',''),('p','888','/reader/payment/rechargeProducts/:id','DELETE','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1701),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1703),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
