-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1708,now(),now(),2,1000,'readerInviteCodes','ReaderInviteCodes',false,'view/reader/inviteCodes/index.vue',25,'',true,false,'邀请码','key',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1708,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1718,now(),now(),'/reader/inviteCodes','查询邀请码','读者运营','GET'),(1719,now(),now(),'/reader/inviteCodes','生成邀请码','读者运营','POST'),(1720,now(),now(),'/reader/inviteCodes/:id/status','启停邀请码','读者运营','PUT'),(1721,now(),now(),'/reader/inviteCodes/:id','删除邀请码','读者运营','DELETE') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/inviteCodes','GET','','',''),('p','888','/reader/inviteCodes','POST','','',''),('p','888','/reader/inviteCodes/:id/status','PUT','','',''),('p','888','/reader/inviteCodes/:id','DELETE','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1708),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1721),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
