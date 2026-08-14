-- +goose Up
-- +goose StatementBegin
CREATE TABLE reader_daily_activity (
    activity_date date NOT NULL,
    reader_id bigint NOT NULL REFERENCES reader_accounts(id),
    first_active_at timestamptz NOT NULL,
    source_type varchar(16) NOT NULL DEFAULT 'runtime',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (activity_date, reader_id),
    CONSTRAINT reader_daily_activity_source_check CHECK (source_type IN ('runtime', 'legacy')),
    CONSTRAINT reader_daily_activity_date_check CHECK (activity_date = (first_active_at AT TIME ZONE 'Asia/Kuala_Lumpur')::date)
);
CREATE INDEX reader_daily_activity_reader_date_idx ON reader_daily_activity(reader_id, activity_date);
CREATE INDEX reader_accounts_created_at_idx ON reader_accounts(created_at, id);

CREATE TABLE reader_activity_settings (
    id smallint PRIMARY KEY,
    tracking_start_date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT reader_activity_settings_singleton_check CHECK (id = 1)
);
INSERT INTO reader_activity_settings(id, tracking_start_date)
VALUES (1, (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kuala_Lumpur')::date);

INSERT INTO sys_apis(id,created_at,updated_at,path,description,api_group,method)
VALUES(1806,now(),now(),'/dashboard/overview','查询读者运营概览','读者运营','GET') ON CONFLICT DO NOTHING;
INSERT INTO casbin_rule(ptype,v0,v1,v2,v3,v4,v5)
VALUES('p','888','/dashboard/overview','GET','','','') ON CONFLICT DO NOTHING;
SELECT setval('sys_apis_id_seq',GREATEST((SELECT max(id) FROM sys_apis),1806),true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
