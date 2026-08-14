-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_base_menus
    (id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,
     active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES
    (1719,now(),now(),2,1000,'novelTxtImports','NovelTxtImports',false,
     'view/novel/txtImports/index.vue',65,'',true,false,'TXT导入','upload',false,'')
ON CONFLICT DO NOTHING;

INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id)
SELECT id,888 FROM sys_base_menus WHERE id=1719 AND path='novelTxtImports'
ON CONFLICT DO NOTHING;

SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1719),true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
