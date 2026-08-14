-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1707,now(),now(),2,1000,'readerFeedback','ReaderFeedback',false,'view/reader/feedback/index.vue',20,'',true,false,'读者反馈','chat-line-square',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1707,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1715,now(),now(),'/reader/feedback','查询读者反馈','读者运营','GET'),(1716,now(),now(),'/reader/feedback/:id','查询反馈详情','读者运营','GET'),(1717,now(),now(),'/reader/feedback/:id/reply','回复读者反馈','读者运营','PUT') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/feedback','GET','','',''),('p','888','/reader/feedback/:id','GET','','',''),('p','888','/reader/feedback/:id/reply','PUT','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1707),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1717),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
