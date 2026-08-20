-- +goose Up
-- +goose StatementBegin
CREATE INDEX reader_checkin_records_audit_date_idx ON reader_checkin_records (checkin_date DESC, id DESC);
CREATE INDEX reader_invite_reward_records_audit_granted_idx ON reader_invite_reward_records (granted_at DESC NULLS LAST, id DESC);
CREATE INDEX reader_invite_reward_records_audit_invitee_idx ON reader_invite_reward_records (invitee_reader_id, granted_at DESC NULLS LAST, id DESC);
CREATE INDEX reader_invite_reward_records_audit_stage_idx ON reader_invite_reward_records (reward_stage, granted_at DESC NULLS LAST, id DESC);
CREATE INDEX reader_invite_reward_records_audit_status_idx ON reader_invite_reward_records (status, granted_at DESC NULLS LAST, id DESC);
INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method) VALUES
    (1810,now(),now(),'/reader/checkins','查询签到记录','读者运营','GET'),
    (1811,now(),now(),'/reader/inviteRewards','查询邀请奖励记录','读者运营','GET') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5) VALUES
    ('p','888','/reader/checkins','GET','','',''),
    ('p','888','/reader/inviteRewards','GET','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1811),true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
