-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1715,now(),now(),2,1000,'novelCrawlBoards','NovelCrawlBoards',false,'view/novel/crawlBoards/index.vue',61,'',true,false,'论坛板块','collection',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1715,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1746,now(),now(),'/novel/crawl/boards','查询论坛板块','内容采集','GET'),(1747,now(),now(),'/novel/crawl/boards','新增论坛板块','内容采集','POST'),(1748,now(),now(),'/novel/crawl/boards/:id','修改论坛板块','内容采集','PUT'),(1749,now(),now(),'/novel/crawl/boards/:id','删除论坛板块','内容采集','DELETE') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/novel/crawl/boards','GET','','',''),('p','888','/novel/crawl/boards','POST','','',''),('p','888','/novel/crawl/boards/:id','PUT','','',''),('p','888','/novel/crawl/boards/:id','DELETE','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1715),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1749),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
