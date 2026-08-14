-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_apis (id,created_at,updated_at,path,description,api_group,method)
VALUES (1797,now(),now(),'/novel/chapterSummary/tasks/:id','查询章节简介补全任务详情','内容生产','GET')
ON CONFLICT (id) DO NOTHING;

INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5)
VALUES ('p','888','/novel/chapterSummary/tasks/:id','GET','','','')
ON CONFLICT DO NOTHING;
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1797),true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
