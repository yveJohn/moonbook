-- +goose Up
-- +goose StatementBegin
MERGE INTO casbin_rule AS target
USING (
    SELECT id
    FROM (
        SELECT id,
               row_number() OVER (
                   PARTITION BY ptype, v0, v1, v2, v3, v4, v5
                   ORDER BY id
               ) AS duplicate_number
        FROM casbin_rule
    ) AS ranked
    WHERE duplicate_number > 1
) AS duplicate
ON target.id = duplicate.id
WHEN MATCHED THEN DELETE;

CREATE UNIQUE INDEX idx_casbin_rule
    ON casbin_rule (ptype, v0, v1, v2, v3, v4, v5);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd
