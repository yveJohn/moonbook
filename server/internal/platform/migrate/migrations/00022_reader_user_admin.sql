-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1706,now(),now(),2,1000,'readerUsers','ReaderUsers',false,'view/reader/users/index.vue',10,'',true,false,'读者用户','user',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1706,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1712,now(),now(),'/reader/users','查询读者用户','读者运营','GET'),(1713,now(),now(),'/reader/users/:id','查询读者详情','读者运营','GET'),(1714,now(),now(),'/reader/users/:id/status','启停读者用户','读者运营','PUT') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/users','GET','','',''),('p','888','/reader/users/:id','GET','','',''),('p','888','/reader/users/:id/status','PUT','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1706),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1714),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
