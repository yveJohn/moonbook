-- +goose Up
-- +goose StatementBegin
CREATE TABLE reader_invite_reward_config (
    id bigint PRIMARY KEY CHECK (id=1),
    enabled boolean NOT NULL DEFAULT false,
    inviter_reward_coin bigint NOT NULL DEFAULT 0,
    invitee_reward_coin bigint NOT NULL DEFAULT 0,
    remark varchar(255) NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT reader_invite_reward_amount_check CHECK (inviter_reward_coin >= 0 AND invitee_reward_coin >= 0)
);
INSERT INTO reader_invite_reward_config(id) VALUES(1) ON CONFLICT DO NOTHING;
INSERT INTO sys_base_menus(id,created_at,updated_at,menu_level,parent_id,path,name,hidden,component,sort,active_name,keep_alive,default_menu,title,icon,close_tab,transition_type)
VALUES(1713,now(),now(),2,1000,'readerInviteReward','ReaderInviteReward',false,'view/reader/inviteReward/index.vue',37,'',true,false,'邀请奖励','medal',false,'') ON CONFLICT DO NOTHING;
INSERT INTO sys_authority_menus(sys_base_menu_id,sys_authority_authority_id) VALUES(1713,888) ON CONFLICT DO NOTHING;
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES(1739,now(),now(),'/reader/inviteReward','查询邀请奖励配置','读者运营','GET'),(1740,now(),now(),'/reader/inviteReward','修改邀请奖励配置','读者运营','PUT') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES('p','888','/reader/inviteReward','GET','','',''),('p','888','/reader/inviteReward','PUT','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_base_menus_id_seq',GREATEST((SELECT max(id) FROM sys_base_menus),1713),true);
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1740),true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
