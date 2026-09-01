-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM reader_invite_reward_records
        GROUP BY invitee_reader_id, reward_stage
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate invite reward stage facts prevent migration';
    END IF;
END $$;

CREATE UNIQUE INDEX reader_invite_reward_records_invitee_stage_uidx
    ON reader_invite_reward_records(invitee_reader_id, reward_stage);

INSERT INTO reader_invite_reward_records(
    relation_id,
    inviter_reader_id,
    invitee_reader_id,
    reward_stage,
    reward_coin,
    status,
    idempotency_key,
    granted_at,
    remark,
    source_type,
    source_ref,
    created_at,
    updated_at
)
SELECT
    relation.id,
    relation.inviter_reader_id,
    relation.invitee_reader_id,
    'register',
    ledger.amount,
    'granted',
    'invite_reward:' || relation.invitee_reader_id::text || ':register',
    ledger.created_at,
    COALESCE(NULLIF(ledger.remark, ''), '邀请注册奖励'),
    'runtime',
    'registration-ledger:' || ledger.id::text,
    ledger.created_at,
    ledger.created_at
FROM reader_wallet_ledgers ledger
JOIN reader_invite_relations relation
  ON ledger.reader_id = relation.inviter_reader_id
 AND ledger.idempotency_key = relation.id::text || ':' || relation.invitee_reader_id::text || ':inviter'
LEFT JOIN reader_invite_reward_records reward
  ON reward.invitee_reader_id = relation.invitee_reader_id
 AND reward.reward_stage = 'register'
WHERE ledger.source_type = 'runtime'
  AND ledger.biz_type = 'invite_reward'
  AND ledger.direction = 'income'
  AND ledger.coin_type = 'bonus'
  AND ledger.amount > 0
  AND ledger.balance_after - ledger.balance_before = ledger.amount
  AND reward.id IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM reader_wallet_ledgers ledger
        LEFT JOIN reader_invite_relations relation
          ON ledger.reader_id = relation.inviter_reader_id
         AND ledger.idempotency_key = relation.id::text || ':' || relation.invitee_reader_id::text || ':inviter'
        LEFT JOIN reader_invite_reward_records reward
          ON reward.invitee_reader_id = relation.invitee_reader_id
         AND reward.reward_stage = 'register'
        WHERE ledger.source_type = 'runtime'
          AND ledger.biz_type = 'invite_reward'
          AND ledger.direction = 'income'
          AND ledger.coin_type = 'bonus'
          AND ledger.idempotency_key LIKE '%:inviter'
          AND (
              relation.id IS NULL
              OR ledger.amount <= 0
              OR ledger.balance_after - ledger.balance_before <> ledger.amount
              OR reward.id IS NULL
              OR reward.relation_id <> relation.id
              OR reward.inviter_reader_id <> relation.inviter_reader_id
              OR reward.invitee_reader_id <> relation.invitee_reader_id
              OR reward.reward_coin <> ledger.amount
              OR reward.status <> 'granted'
          )
    ) THEN
        RAISE EXCEPTION 'unreconciled runtime invite registration reward ledger prevents migration';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
