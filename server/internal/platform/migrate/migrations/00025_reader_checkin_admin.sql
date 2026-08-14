-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1709,now(),now(),2,1000,'readerCheckinRules','ReaderCheckinRules',false,'view/reader/checkinRules/index.vue',26,'',true,false,'签到奖励规则','calendar',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1709,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES
(1722,now(),now(),'/reader/checkinRules','查询签到奖励规则','读者运营','GET'),
(1723,now(),now(),'/reader/checkinRules','创建签到奖励规则','读者运营','POST'),
(1724,now(),now(),'/reader/checkinRules/:id','更新签到奖励规则','读者运营','PUT'),
(1725,now(),now(),'/reader/checkinRules/:id','删除签到奖励规则','读者运营','DELETE') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES
('p','888','/reader/checkinRules','GET','','',''),('p','888','/reader/checkinRules','POST','','',''),
('p','888','/reader/checkinRules/:id','PUT','','',''),('p','888','/reader/checkinRules/:id','DELETE','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1709),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1725),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
