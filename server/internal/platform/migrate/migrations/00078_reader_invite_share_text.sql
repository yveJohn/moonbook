-- +goose Up
-- +goose StatementBegin
INSERT INTO sys_params(created_at, updated_at, name, key, value, "desc")
SELECT now(), now(),
       '读者邀请分享文案',
       'reader.invite.shareText',
       '邀请你加入月白书城，点击 {{link}} 注册',
       '读者端复制邀请文案，{{link}} 会替换为当前域名的邀请注册链接'
WHERE NOT EXISTS (
    SELECT 1
    FROM sys_params
    WHERE key = 'reader.invite.shareText'
      AND deleted_at IS NULL
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
