package legacymigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// ReaderFinanceStage migrates financial facts after reader identity and
// commerce references exist. Source tables are read by increasing bigint ID.
type ReaderFinanceStage struct{}

func (ReaderFinanceStage) Name() string { return "reader-finance" }

var readerFinanceTables = []string{
	"reader_wallet",
	"reader_wallet_ledger",
	"reader_bonus_coin_bucket",
	"reader_order",
	"reader_checkin_reward_rule",
	"reader_checkin_record",
	"reader_invite_reward_record",
	"reader_wallet_adjustment",
	"reader_recharge_product",
	"reader_recharge_setting",
	"reader_payment_channel",
	"reader_recharge_order",
	"reader_payment_callback_log",
}

func (ReaderFinanceStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, last, err := financeCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	for i := indexOf(readerFinanceTables, table); i < len(readerFinanceTables); i++ {
		table = readerFinanceTables[i]
		exists, err := sourceTableExists(ctx, source, table)
		if err != nil {
			return BatchResult{}, err
		}
		if !exists {
			last = 0
			continue
		}
		return runFinanceTable(ctx, source, target, table, last, limit)
	}
	return BatchResult{Done: true, NextCursor: "reader_payment_callback_log:0", Metadata: map[string]any{"source": "reader finance", "skipped": "table_not_found"}}, nil
}

func financeCursor(raw string) (string, int64, error) {
	if strings.TrimSpace(raw) == "" {
		return readerFinanceTables[0], 0, nil
	}
	table, last, err := stageCursor(raw, readerFinanceTables[0])
	if err != nil {
		return "", 0, err
	}
	if indexOf(readerFinanceTables, table) == 0 && table != readerFinanceTables[0] {
		return "", 0, fmt.Errorf("invalid reader finance cursor table %q", table)
	}
	return table, last, nil
}

func runFinanceTable(ctx context.Context, source *sql.DB, target *sql.Tx, table string, last int64, limit int) (BatchResult, error) {
	queries := map[string]string{
		"reader_wallet":               `SELECT reader_id,recharge_coin_balance,bonus_coin_balance,total_recharge_coin_income,total_bonus_coin_income,total_recharge_coin_expense,total_bonus_coin_expense,create_time,update_time FROM reader_wallet WHERE reader_id>? ORDER BY reader_id LIMIT ?`,
		"reader_wallet_ledger":        `SELECT id,reader_id,ledger_no,idempotency_key,biz_type,biz_id,order_no,direction,coin_type,amount,balance_before,balance_after,remark,create_time FROM reader_wallet_ledger WHERE id>? ORDER BY id LIMIT ?`,
		"reader_bonus_coin_bucket":    `SELECT id,reader_id,source_type,source_id,original_amount,remaining_amount,expire_time,status,create_time,update_time FROM reader_bonus_coin_bucket WHERE id>? ORDER BY id LIMIT ?`,
		"reader_order":                `SELECT id,order_no,reader_id,order_type,product_id,product_type,target_id,book_id_snapshot,product_name_snapshot,price_coin_snapshot,chapter_word_count_snapshot,pricing_word_unit_snapshot,pricing_coin_unit_snapshot,recharge_coin_amount,bonus_coin_amount,status,idempotency_key,remark,operator_id,paid_time,create_time,update_time FROM reader_order WHERE id>? ORDER BY id LIMIT ?`,
		"reader_checkin_reward_rule":  `SELECT id,rule_type,continuous_days,reward_mode,fixed_coin,min_coin,max_coin,status,sort_order,remark,create_time,update_time FROM reader_checkin_reward_rule WHERE id>? ORDER BY id LIMIT ?`,
		"reader_checkin_record":       `SELECT id,reader_id,checkin_date,continuous_days,base_reward_coin,milestone_reward_coin,total_reward_coin,idempotency_key,create_time FROM reader_checkin_record WHERE id>? ORDER BY id LIMIT ?`,
		"reader_invite_reward_record": `SELECT id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,grant_time,remark,create_time,update_time FROM reader_invite_reward_record WHERE id>? ORDER BY id LIMIT ?`,
		"reader_wallet_adjustment":    `SELECT id,adjustment_no,reader_id,coin_type,direction,amount,reason,status,idempotency_key,operator_id,operator_name,ledger_id,create_time,update_time FROM reader_wallet_adjustment WHERE id>? ORDER BY id LIMIT ?`,
		"reader_recharge_product":     `SELECT id,product_name,diamond_amount,price_usdt,sale_status,sort_order,create_time,update_time FROM reader_recharge_product WHERE id>? ORDER BY id LIMIT ?`,
		"reader_recharge_setting":     `SELECT id,custom_enabled,diamonds_per_usdt,min_diamond_amount,max_diamond_amount,amount_scale,rounding_mode,update_time FROM reader_recharge_setting WHERE id>? ORDER BY id LIMIT ?`,
		"reader_payment_channel":      `SELECT id,provider,enabled,currency,token,network,update_time FROM reader_payment_channel WHERE id>? ORDER BY id LIMIT ?`,
		"reader_recharge_order":       `SELECT id,order_no,reader_id,request_id,source_type,product_id,diamond_amount,price_usdt,provider,channel_id,payment_credential_id,merchant_pid_snapshot,currency,token,network,gateway_trade_id,gateway_actual_amount,receive_address,payment_url,block_transaction_id,status,gateway_status,wallet_ledger_id,expire_time,paid_time,failure_code,failure_message,create_time,update_time FROM reader_recharge_order WHERE id>? ORDER BY id LIMIT ?`,
		"reader_payment_callback_log": `SELECT id,provider,recharge_order_id,merchant_order_no,gateway_trade_id,source_ip,payload_hash,payload_snapshot,signature_valid,processing_result,failure_reason,response_status,response_body,request_time,create_time FROM reader_payment_callback_log WHERE id>? ORDER BY id LIMIT ?`,
	}
	rows, err := source.QueryContext(ctx, queries[table], last, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: fmt.Sprintf("%s:%d", table, last), Metadata: map[string]any{"source": table}}
	for rows.Next() {
		id, recordError, err := migrateFinanceRow(ctx, target, table, rows)
		if err != nil {
			return BatchResult{}, err
		}
		if recordError != nil {
			result.Errors = append(result.Errors, *recordError)
		}
		result.Processed++
		result.NextCursor = fmt.Sprintf("%s:%d", table, id)
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	if result.Done {
		if err := syncReaderFinanceSequence(ctx, target, table); err != nil {
			return BatchResult{}, err
		}
		if next := nextFinanceCursor(table); next != "" {
			result.Done = false
			result.NextCursor = next
		} else if err := reconcileMigratedWallets(ctx, target); err != nil {
			return BatchResult{}, err
		}
	}
	return result, nil
}

func migrateFinanceRow(ctx context.Context, target *sql.Tx, table string, rows *sql.Rows) (int64, *RecordError, error) {
	sourceRef := func(id int64) string { return strconv.FormatInt(id, 10) }
	switch table {
	case "reader_wallet":
		var id, recharge, bonus, rechargeIncome, bonusIncome, rechargeExpense, bonusExpense int64
		var created, updated sql.NullTime
		if err := rows.Scan(&id, &recharge, &bonus, &rechargeIncome, &bonusIncome, &rechargeExpense, &bonusExpense, &created, &updated); err != nil {
			return 0, nil, err
		}
		if recharge < 0 || bonus < 0 || rechargeIncome < 0 || bonusIncome < 0 || rechargeExpense < 0 || bonusExpense < 0 {
			return id, migrationRecordError(table, id, "INVALID_WALLET_TOTAL", "legacy wallet contains a negative balance or total"), nil
		}
		if ok, err := financeReferenceExists(ctx, target, "reader_accounts", id); err != nil || !ok {
			return financeReferenceError(table, id, "MISSING_READER", "legacy wallet references an unknown reader", err)
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,recharge_coin_balance,bonus_coin_balance,total_recharge_coin_income,total_bonus_coin_income,total_recharge_coin_expense,total_bonus_coin_expense,source_type,source_ref,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,'legacy',$8,COALESCE($9,now()),COALESCE($10,now())) ON CONFLICT(reader_id) DO UPDATE SET recharge_coin_balance=EXCLUDED.recharge_coin_balance,bonus_coin_balance=EXCLUDED.bonus_coin_balance,total_recharge_coin_income=EXCLUDED.total_recharge_coin_income,total_bonus_coin_income=EXCLUDED.total_bonus_coin_income,total_recharge_coin_expense=EXCLUDED.total_recharge_coin_expense,total_bonus_coin_expense=EXCLUDED.total_bonus_coin_expense,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, recharge, bonus, rechargeIncome, bonusIncome, rechargeExpense, bonusExpense, sourceRef(id), nullableLegacyTime(created), nullableLegacyTime(updated))
		return id, nil, err

	case "reader_wallet_ledger":
		var id, readerID, amount, before, after int64
		var ledgerNo, key, bizType, direction, coinType, remark string
		var bizID sql.NullInt64
		var orderNo sql.NullString
		var created sql.NullTime
		if err := rows.Scan(&id, &readerID, &ledgerNo, &key, &bizType, &bizID, &orderNo, &direction, &coinType, &amount, &before, &after, &remark, &created); err != nil {
			return 0, nil, err
		}
		if !oneOf(direction, "income", "expense") || !oneOf(coinType, "recharge", "bonus") || amount <= 0 || before < 0 || after < 0 || (direction == "income" && after-before != amount) || (direction == "expense" && before-after != amount) {
			return id, migrationRecordError(table, id, "INVALID_WALLET_LEDGER", "legacy wallet ledger direction, coin type, amount, or balance transition is invalid"), nil
		}
		if ok, err := financeReferenceExists(ctx, target, "reader_accounts", readerID); err != nil || !ok {
			return financeReferenceError(table, id, "MISSING_READER", "legacy wallet ledger references an unknown reader", err)
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_wallet_ledgers(id,reader_id,ledger_no,biz_type,biz_id,order_no,direction,coin_type,amount,balance_before,balance_after,remark,idempotency_key,source_type,source_ref,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'legacy',$14,COALESCE($15,now())) ON CONFLICT(id) DO NOTHING`, id, readerID, ledgerNo, bizType, nullableInt64String(bizID), nullableString(orderNo), direction, coinType, amount, before, after, remark, key, sourceRef(id), nullableLegacyTime(created))
		return id, nil, err

	case "reader_bonus_coin_bucket":
		var id, readerID, original, remaining int64
		var sourceKind, status string
		var sourceID sql.NullInt64
		var expires, created, updated sql.NullTime
		if err := rows.Scan(&id, &readerID, &sourceKind, &sourceID, &original, &remaining, &expires, &status, &created, &updated); err != nil {
			return 0, nil, err
		}
		if !oneOf(status, "active", "depleted", "expired") || original < 0 || remaining < 0 || remaining > original || !expires.Valid {
			return id, migrationRecordError(table, id, "INVALID_BONUS_BUCKET", "legacy bonus bucket amount, expiry, or status is invalid"), nil
		}
		if ok, err := financeReferenceExists(ctx, target, "reader_accounts", readerID); err != nil || !ok {
			return financeReferenceError(table, id, "MISSING_READER", "legacy bonus bucket references an unknown reader", err)
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_bonus_coin_buckets(id,reader_id,source_kind,source_id,original_amount,remaining_amount,expires_at,status,source_type,source_ref,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'legacy',$9,COALESCE($10,now()),COALESCE($11,now())) ON CONFLICT(id) DO UPDATE SET remaining_amount=EXCLUDED.remaining_amount,status=EXCLUDED.status,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, readerID, sourceKind, nullableInt64(sourceID), original, remaining, expires.Time, status, sourceRef(id), nullableLegacyTime(created), nullableLegacyTime(updated))
		return id, nil, err

	case "reader_order":
		return migrateLegacyPurchaseOrder(ctx, target, rows)
	case "reader_checkin_reward_rule":
		return migrateLegacyCheckinRule(ctx, target, rows)
	case "reader_checkin_record":
		var id, readerID, days, base, milestone, total int64
		var date, created sql.NullTime
		var key string
		if err := rows.Scan(&id, &readerID, &date, &days, &base, &milestone, &total, &key, &created); err != nil {
			return 0, nil, err
		}
		if !date.Valid || days <= 0 || base < 0 || milestone < 0 || total != base+milestone {
			return id, migrationRecordError(table, id, "INVALID_CHECKIN_RECORD", "legacy check-in date, days, or reward totals are invalid"), nil
		}
		if ok, err := financeReferenceExists(ctx, target, "reader_accounts", readerID); err != nil || !ok {
			return financeReferenceError(table, id, "MISSING_READER", "legacy check-in record references an unknown reader", err)
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_checkin_records(id,reader_id,checkin_date,continuous_days,base_reward_coin,milestone_reward_coin,total_reward_coin,idempotency_key,source_type,source_ref,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'legacy',$9,COALESCE($10,now())) ON CONFLICT(id) DO UPDATE SET continuous_days=EXCLUDED.continuous_days,base_reward_coin=EXCLUDED.base_reward_coin,milestone_reward_coin=EXCLUDED.milestone_reward_coin,total_reward_coin=EXCLUDED.total_reward_coin,source_type='legacy',source_ref=EXCLUDED.source_ref`, id, readerID, date.Time, days, base, milestone, total, key, sourceRef(id), nullableLegacyTime(created))
		return id, nil, err

	case "reader_invite_reward_record":
		var id, relationID, inviterID, inviteeID, reward int64
		var stage, status, key, remark string
		var granted, created, updated sql.NullTime
		if err := rows.Scan(&id, &relationID, &inviterID, &inviteeID, &stage, &reward, &status, &key, &granted, &remark, &created, &updated); err != nil {
			return 0, nil, err
		}
		if !oneOf(stage, "register", "first_recharge") || !oneOf(status, "granted", "skipped", "failed") || reward < 0 {
			return id, migrationRecordError(table, id, "INVALID_INVITE_REWARD", "legacy invite reward stage, status, or amount is invalid"), nil
		}
		if ok, err := financeReferenceExists(ctx, target, "reader_invite_relations", relationID); err != nil || !ok {
			return financeReferenceError(table, id, "MISSING_INVITE_RELATION", "legacy invite reward references an unknown relation", err)
		}
		var relationMatches bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reader_invite_relations WHERE id=$1 AND inviter_reader_id=$2 AND invitee_reader_id=$3)`, relationID, inviterID, inviteeID).Scan(&relationMatches); err != nil {
			return id, nil, err
		}
		if !relationMatches {
			return id, migrationRecordError(table, id, "INVITE_RELATION_MISMATCH", "legacy invite reward reader ids do not match the invite relation"), nil
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_invite_reward_records(id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at,remark,source_type,source_ref,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'legacy',$11,COALESCE($12,now()),COALESCE($13,now())) ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status,granted_at=EXCLUDED.granted_at,remark=EXCLUDED.remark,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, relationID, inviterID, inviteeID, stage, reward, status, key, nullableLegacyTime(granted), remark, sourceRef(id), nullableLegacyTime(created), nullableLegacyTime(updated))
		return id, nil, err

	case "reader_wallet_adjustment":
		var id, readerID, amount int64
		var no, coinType, direction, reason, status, key, operatorName string
		var operatorID, ledgerID sql.NullInt64
		var created, updated sql.NullTime
		if err := rows.Scan(&id, &no, &readerID, &coinType, &direction, &amount, &reason, &status, &key, &operatorID, &operatorName, &ledgerID, &created, &updated); err != nil {
			return 0, nil, err
		}
		if !oneOf(coinType, "recharge", "bonus") || !oneOf(direction, "increase", "decrease") || !oneOf(status, "confirmed", "failed") || amount <= 0 {
			return id, migrationRecordError(table, id, "INVALID_WALLET_ADJUSTMENT", "legacy wallet adjustment fields are invalid"), nil
		}
		if ledgerID.Valid {
			if ok, err := financeReferenceExists(ctx, target, "reader_wallet_ledgers", ledgerID.Int64); err != nil || !ok {
				return financeReferenceError(table, id, "MISSING_WALLET_LEDGER", "legacy wallet adjustment references an unknown ledger", err)
			}
		}
		if ok, err := financeReferenceExists(ctx, target, "reader_accounts", readerID); err != nil || !ok {
			return financeReferenceError(table, id, "MISSING_READER", "legacy wallet adjustment references an unknown reader", err)
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_wallet_adjustments(id,adjustment_no,reader_id,coin_type,direction,amount,reason,status,idempotency_key,operator_id,operator_name,ledger_id,source_type,source_ref,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'legacy',$13,COALESCE($14,now()),COALESCE($15,now())) ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, no, readerID, coinType, direction, amount, reason, status, key, nullableInt64(operatorID), operatorName, nullableInt64(ledgerID), sourceRef(id), nullableLegacyTime(created), nullableLegacyTime(updated))
		return id, nil, err

	case "reader_recharge_product":
		var id, diamonds int64
		var name, price, status string
		var sortOrder int
		var created, updated sql.NullTime
		if err := rows.Scan(&id, &name, &diamonds, &price, &status, &sortOrder, &created, &updated); err != nil {
			return 0, nil, err
		}
		if diamonds <= 0 || !positiveDecimal(price) || !oneOf(status, "on_sale", "off_sale") {
			return id, migrationRecordError(table, id, "INVALID_RECHARGE_PRODUCT", "legacy recharge product amount or status is invalid"), nil
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_recharge_products(id,product_name,diamond_amount,price_usdt,sale_status,sort_order,source_type,source_ref,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'legacy',$7,COALESCE($8,now()),COALESCE($9,now())) ON CONFLICT(id) DO UPDATE SET product_name=EXCLUDED.product_name,diamond_amount=EXCLUDED.diamond_amount,price_usdt=EXCLUDED.price_usdt,sale_status=EXCLUDED.sale_status,sort_order=EXCLUDED.sort_order,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, name, diamonds, price, status, sortOrder, sourceRef(id), nullableLegacyTime(created), nullableLegacyTime(updated))
		return id, nil, err

	case "reader_recharge_setting":
		var id, minAmount, maxAmount int64
		var enabled bool
		var rate, rounding string
		var scale int
		var updated sql.NullTime
		if err := rows.Scan(&id, &enabled, &rate, &minAmount, &maxAmount, &scale, &rounding, &updated); err != nil {
			return 0, nil, err
		}
		if id != 1 || !positiveDecimal(rate) || minAmount <= 0 || maxAmount < minAmount || scale < 0 || scale > 8 || !oneOf(rounding, "CEILING") {
			return id, migrationRecordError(table, id, "INVALID_RECHARGE_SETTING", "legacy recharge setting fields are invalid"), nil
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_recharge_settings(id,custom_enabled,diamonds_per_usdt,min_diamond_amount,max_diamond_amount,amount_scale,rounding_mode,source_type,source_ref,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,'legacy',$8,COALESCE($9,now())) ON CONFLICT(id) DO UPDATE SET custom_enabled=EXCLUDED.custom_enabled,diamonds_per_usdt=EXCLUDED.diamonds_per_usdt,min_diamond_amount=EXCLUDED.min_diamond_amount,max_diamond_amount=EXCLUDED.max_diamond_amount,amount_scale=EXCLUDED.amount_scale,rounding_mode=EXCLUDED.rounding_mode,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, enabled, rate, minAmount, maxAmount, scale, rounding, sourceRef(id), nullableLegacyTime(updated))
		return id, nil, err

	case "reader_payment_channel":
		var id int64
		var provider, currency, token, network string
		var enabled bool
		var updated sql.NullTime
		if err := rows.Scan(&id, &provider, &enabled, &currency, &token, &network, &updated); err != nil {
			return 0, nil, err
		}
		if strings.TrimSpace(provider) == "" || strings.TrimSpace(currency) == "" || strings.TrimSpace(token) == "" || strings.TrimSpace(network) == "" {
			return id, migrationRecordError(table, id, "INVALID_PAYMENT_CHANNEL", "legacy payment channel contains an empty required field"), nil
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_payment_channels(id,provider,enabled,currency,token,network,source_type,source_ref,updated_at) VALUES($1,$2,$3,$4,$5,$6,'legacy',$7,COALESCE($8,now())) ON CONFLICT(id) DO UPDATE SET provider=EXCLUDED.provider,enabled=EXCLUDED.enabled,currency=EXCLUDED.currency,token=EXCLUDED.token,network=EXCLUDED.network,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, provider, enabled, currency, token, network, sourceRef(id), nullableLegacyTime(updated))
		return id, nil, err

	case "reader_recharge_order":
		return migrateLegacyRechargeOrder(ctx, target, rows)
	case "reader_payment_callback_log":
		return migrateLegacyPaymentCallback(ctx, target, rows)
	default:
		return 0, nil, fmt.Errorf("unsupported reader finance table %q", table)
	}
}

func migrateLegacyPurchaseOrder(ctx context.Context, target *sql.Tx, rows *sql.Rows) (int64, *RecordError, error) {
	var id, readerID, price, recharge, bonus int64
	var orderNo, legacyType, name, status, key, remark string
	var productID, targetID, bookID, pricingCoin, operatorID sql.NullInt64
	var productType sql.NullString
	var wordCount, wordUnit sql.NullInt64
	var paid, created, updated sql.NullTime
	if err := rows.Scan(&id, &orderNo, &readerID, &legacyType, &productID, &productType, &targetID, &bookID, &name, &price, &wordCount, &wordUnit, &pricingCoin, &recharge, &bonus, &status, &key, &remark, &operatorID, &paid, &created, &updated); err != nil {
		return 0, nil, err
	}
	typeMap := map[string]string{"buy_book": "book", "buy_chapter": "chapter", "buy_ad_free": "ad_free", "buy_membership": "membership", "mock_recharge": "mock_recharge"}
	orderType, ok := typeMap[strings.TrimSpace(legacyType)]
	if !ok || !oneOf(status, "pending", "paid", "closed", "failed") || price < 0 || recharge < 0 || bonus < 0 {
		return id, migrationRecordError("reader_order", id, "INVALID_PURCHASE_ORDER", "legacy purchase order type, status, or amount is invalid"), nil
	}
	if ok, err := financeReferenceExists(ctx, target, "reader_accounts", readerID); err != nil || !ok {
		return financeReferenceError("reader_order", id, "MISSING_READER", "legacy purchase order references an unknown reader", err)
	}
	if productID.Valid {
		if ok, err := financeReferenceExists(ctx, target, "commerce_products", productID.Int64); err != nil || !ok {
			return financeReferenceError("reader_order", id, "MISSING_PRODUCT", "legacy purchase order references an unknown product", err)
		}
	}
	if !productType.Valid || strings.TrimSpace(productType.String) == "" {
		productType = sql.NullString{String: orderType, Valid: true}
	}
	if !oneOf(productType.String, "book", "chapter", "ad_free", "membership", "mock_recharge") {
		return id, migrationRecordError("reader_order", id, "INVALID_PRODUCT_TYPE", "legacy purchase order product type is invalid"), nil
	}
	_, err := target.ExecContext(ctx, `INSERT INTO reader_purchase_orders(id,order_no,reader_id,order_type,product_id,product_type,target_id,book_id_snapshot,product_name_snapshot,price_coin_snapshot,chapter_word_count_snapshot,pricing_word_unit_snapshot,pricing_coin_unit_snapshot,recharge_coin_amount,bonus_coin_amount,status,idempotency_key,remark,operator_id,source_type,source_ref,paid_time,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,'legacy',$20,$21,COALESCE($22,now()),COALESCE($23,now())) ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status,remark=EXCLUDED.remark,operator_id=EXCLUDED.operator_id,source_type='legacy',source_ref=EXCLUDED.source_ref,paid_time=EXCLUDED.paid_time,updated_at=EXCLUDED.updated_at`, id, orderNo, readerID, orderType, nullableInt64(productID), nullableString(productType), nullableInt64(targetID), nullableInt64(bookID), name, price, nullableInt64(wordCount), nullableInt64(wordUnit), nullableInt64(pricingCoin), recharge, bonus, status, key, remark, nullableInt64(operatorID), strconv.FormatInt(id, 10), nullableLegacyTime(paid), nullableLegacyTime(created), nullableLegacyTime(updated))
	return id, nil, err
}

func migrateLegacyCheckinRule(ctx context.Context, target *sql.Tx, rows *sql.Rows) (int64, *RecordError, error) {
	var id int64
	var ruleType, mode, status, remark string
	var days, fixed, minCoin, maxCoin sql.NullInt64
	var sortOrder int
	var created, updated sql.NullTime
	if err := rows.Scan(&id, &ruleType, &days, &mode, &fixed, &minCoin, &maxCoin, &status, &sortOrder, &remark, &created, &updated); err != nil {
		return 0, nil, err
	}
	validShape := (ruleType == "daily" && !days.Valid) || (ruleType == "continuous" && days.Valid && days.Int64 > 0)
	validReward := (mode == "fixed" && fixed.Valid && fixed.Int64 > 0 && !minCoin.Valid && !maxCoin.Valid) || (mode == "random" && !fixed.Valid && minCoin.Valid && minCoin.Int64 > 0 && maxCoin.Valid && maxCoin.Int64 >= minCoin.Int64)
	if !validShape || !validReward || !oneOf(status, "enabled", "disabled") {
		return id, migrationRecordError("reader_checkin_reward_rule", id, "INVALID_CHECKIN_RULE", "legacy check-in rule shape, reward, or status is invalid"), nil
	}
	_, err := target.ExecContext(ctx, `INSERT INTO reader_checkin_reward_rules(id,rule_type,continuous_days,reward_mode,fixed_coin,min_coin,max_coin,status,sort_order,remark,source_type,source_ref,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'legacy',$11,COALESCE($12,now()),COALESCE($13,now())) ON CONFLICT(id) DO UPDATE SET rule_type=EXCLUDED.rule_type,continuous_days=EXCLUDED.continuous_days,reward_mode=EXCLUDED.reward_mode,fixed_coin=EXCLUDED.fixed_coin,min_coin=EXCLUDED.min_coin,max_coin=EXCLUDED.max_coin,status=EXCLUDED.status,sort_order=EXCLUDED.sort_order,remark=EXCLUDED.remark,source_type='legacy',source_ref=EXCLUDED.source_ref,updated_at=EXCLUDED.updated_at`, id, ruleType, nullableInt64(days), mode, nullableInt64(fixed), nullableInt64(minCoin), nullableInt64(maxCoin), status, sortOrder, remark, strconv.FormatInt(id, 10), nullableLegacyTime(created), nullableLegacyTime(updated))
	return id, nil, err
}

func migrateLegacyRechargeOrder(ctx context.Context, target *sql.Tx, rows *sql.Rows) (int64, *RecordError, error) {
	var id, readerID, diamonds, channelID, credentialID int64
	var orderNo, requestID, sourceType, price, provider, merchantPID, currency, token, network, status, failureCode, failureMessage string
	var productID, walletLedgerID, gatewayStatus sql.NullInt64
	var gatewayTradeID, actualAmount, receiveAddress, paymentURL, blockTransactionID sql.NullString
	var expires, paid, created, updated sql.NullTime
	if err := rows.Scan(&id, &orderNo, &readerID, &requestID, &sourceType, &productID, &diamonds, &price, &provider, &channelID, &credentialID, &merchantPID, &currency, &token, &network, &gatewayTradeID, &actualAmount, &receiveAddress, &paymentURL, &blockTransactionID, &status, &gatewayStatus, &walletLedgerID, &expires, &paid, &failureCode, &failureMessage, &created, &updated); err != nil {
		return 0, nil, err
	}
	if !oneOf(sourceType, "preset", "custom") || !oneOf(status, "creating", "pending", "gateway_unknown", "create_failed", "superseded", "expired", "callback_exception", "paid") || diamonds <= 0 || !positiveDecimal(price) {
		return id, migrationRecordError("reader_recharge_order", id, "INVALID_RECHARGE_ORDER", "legacy recharge order source, status, or amount is invalid"), nil
	}
	if ok, err := financeReferenceExists(ctx, target, "reader_accounts", readerID); err != nil || !ok {
		return financeReferenceError("reader_recharge_order", id, "MISSING_READER", "legacy recharge order references an unknown reader", err)
	}
	if productID.Valid {
		if ok, err := financeReferenceExists(ctx, target, "reader_recharge_products", productID.Int64); err != nil || !ok {
			return financeReferenceError("reader_recharge_order", id, "MISSING_RECHARGE_PRODUCT", "legacy recharge order references an unknown product", err)
		}
	}
	if walletLedgerID.Valid {
		if ok, err := financeReferenceExists(ctx, target, "reader_wallet_ledgers", walletLedgerID.Int64); err != nil || !ok {
			return financeReferenceError("reader_recharge_order", id, "MISSING_WALLET_LEDGER", "legacy recharge order references an unknown ledger", err)
		}
	}
	_, err := target.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,product_id,diamond_amount,price_usdt,provider,currency,token,network,gateway_trade_id,actual_amount,receive_address,payment_url,block_transaction_id,status,gateway_status,wallet_ledger_id,expire_time,paid_time,failure_code,failure_message,legacy_channel_id,legacy_payment_credential_id,merchant_pid_sha256,legacy_source_ref,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,COALESCE($15,''),COALESCE($16,''),$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,COALESCE($29,now()),COALESCE($30,now())) ON CONFLICT(id) DO UPDATE SET gateway_trade_id=EXCLUDED.gateway_trade_id,actual_amount=EXCLUDED.actual_amount,receive_address=EXCLUDED.receive_address,payment_url=EXCLUDED.payment_url,block_transaction_id=EXCLUDED.block_transaction_id,status=EXCLUDED.status,gateway_status=EXCLUDED.gateway_status,wallet_ledger_id=EXCLUDED.wallet_ledger_id,expire_time=EXCLUDED.expire_time,paid_time=EXCLUDED.paid_time,failure_code=EXCLUDED.failure_code,failure_message=EXCLUDED.failure_message,legacy_channel_id=EXCLUDED.legacy_channel_id,legacy_payment_credential_id=EXCLUDED.legacy_payment_credential_id,merchant_pid_sha256=EXCLUDED.merchant_pid_sha256,legacy_source_ref=EXCLUDED.legacy_source_ref,updated_at=EXCLUDED.updated_at`, id, orderNo, readerID, requestID, sourceType, nullableInt64(productID), diamonds, price, provider, currency, token, network, nullableString(gatewayTradeID), nullableString(actualAmount), nullableString(receiveAddress), nullableString(paymentURL), nullableString(blockTransactionID), status, nullableInt64(gatewayStatus), nullableInt64(walletLedgerID), nullableLegacyTime(expires), nullableLegacyTime(paid), failureCode, failureMessage, channelID, credentialID, sha256Text(merchantPID), strconv.FormatInt(id, 10), nullableLegacyTime(created), nullableLegacyTime(updated))
	return id, nil, err
}

func migrateLegacyPaymentCallback(ctx context.Context, target *sql.Tx, rows *sql.Rows) (int64, *RecordError, error) {
	var id int64
	var provider, merchantOrderNo, gatewayTradeID, sourceIP, payloadHash, result, reason, responseBody string
	var rechargeOrderID sql.NullInt64
	var payload sql.NullString
	var signatureValid bool
	var responseStatus int
	var requested, created sql.NullTime
	if err := rows.Scan(&id, &provider, &rechargeOrderID, &merchantOrderNo, &gatewayTradeID, &sourceIP, &payloadHash, &payload, &signatureValid, &result, &reason, &responseStatus, &responseBody, &requested, &created); err != nil {
		return 0, nil, err
	}
	if !oneOf(result, "success", "rejected", "failed") || !requested.Valid || !validSHA256(payloadHash) || (payload.Valid && !json.Valid([]byte(payload.String))) {
		return id, migrationRecordError("reader_payment_callback_log", id, "INVALID_PAYMENT_CALLBACK", "legacy callback result, request time, or JSON snapshot is invalid"), nil
	}
	if rechargeOrderID.Valid {
		if ok, err := financeReferenceExists(ctx, target, "reader_recharge_orders", rechargeOrderID.Int64); err != nil || !ok {
			return financeReferenceError("reader_payment_callback_log", id, "MISSING_RECHARGE_ORDER", "legacy callback references an unknown recharge order", err)
		}
	}
	_, err := target.ExecContext(ctx, `INSERT INTO reader_payment_callback_logs(id,provider,recharge_order_id,merchant_order_no,gateway_trade_id,payload_hash,payload_snapshot,signature_valid,processing_result,failure_reason,response_status,response_body,request_time,source_ip_sha256,source_type,source_ref,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'legacy',$15,COALESCE($16,now())) ON CONFLICT(id) DO UPDATE SET processing_result=EXCLUDED.processing_result,failure_reason=EXCLUDED.failure_reason,response_status=EXCLUDED.response_status,response_body=EXCLUDED.response_body,source_ip_sha256=EXCLUDED.source_ip_sha256,source_type='legacy',source_ref=EXCLUDED.source_ref`, id, provider, nullableInt64(rechargeOrderID), merchantOrderNo, gatewayTradeID, payloadHash, nullableString(payload), signatureValid, result, reason, responseStatus, responseBody, requested.Time, sha256Text(sourceIP), strconv.FormatInt(id, 10), nullableLegacyTime(created))
	return id, nil, err
}

func financeReferenceExists(ctx context.Context, target *sql.Tx, table string, id int64) (bool, error) {
	allowed := map[string]bool{"reader_accounts": true, "reader_invite_relations": true, "reader_wallet_ledgers": true, "reader_recharge_products": true, "reader_recharge_orders": true, "commerce_products": true}
	if !allowed[table] {
		return false, fmt.Errorf("unsupported finance reference table %q", table)
	}
	var exists bool
	err := target.QueryRowContext(ctx, fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id=$1)", table), id).Scan(&exists)
	return exists, err
}

func financeReferenceError(table string, id int64, code, message string, err error) (int64, *RecordError, error) {
	if err != nil {
		return id, nil, err
	}
	return id, migrationRecordError(table, id, code, message), nil
}

func nullableInt64String(value sql.NullInt64) any {
	if value.Valid {
		return strconv.FormatInt(value.Int64, 10)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	value = strings.TrimSpace(value)
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func positiveDecimal(value string) bool {
	number, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	return ok && number.Sign() > 0
}

func sha256Text(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func syncReaderFinanceSequence(ctx context.Context, target *sql.Tx, sourceTable string) error {
	tables := map[string]string{
		"reader_wallet_ledger":        "reader_wallet_ledgers",
		"reader_bonus_coin_bucket":    "reader_bonus_coin_buckets",
		"reader_order":                "reader_purchase_orders",
		"reader_checkin_reward_rule":  "reader_checkin_reward_rules",
		"reader_checkin_record":       "reader_checkin_records",
		"reader_invite_reward_record": "reader_invite_reward_records",
		"reader_wallet_adjustment":    "reader_wallet_adjustments",
		"reader_recharge_order":       "reader_recharge_orders",
		"reader_payment_callback_log": "reader_payment_callback_logs",
	}
	if table, ok := tables[sourceTable]; ok {
		return syncIdentitySequence(ctx, target, table)
	}
	return nil
}

func nextFinanceCursor(table string) string {
	for i, value := range readerFinanceTables {
		if value == table && i+1 < len(readerFinanceTables) {
			return fmt.Sprintf("%s:0", readerFinanceTables[i+1])
		}
	}
	return ""
}

func reconcileMigratedWallets(ctx context.Context, target *sql.Tx) error {
	rows, err := target.QueryContext(ctx, `
		SELECT w.reader_id
		FROM reader_wallets w
		LEFT JOIN LATERAL (
			SELECT
				COALESCE(sum(amount) FILTER (WHERE coin_type='recharge' AND direction='income'),0) AS recharge_income,
				COALESCE(sum(amount) FILTER (WHERE coin_type='recharge' AND direction='expense'),0) AS recharge_expense,
				COALESCE(sum(amount) FILTER (WHERE coin_type='bonus' AND direction='income'),0) AS bonus_income,
				COALESCE(sum(amount) FILTER (WHERE coin_type='bonus' AND direction='expense'),0) AS bonus_expense
			FROM reader_wallet_ledgers l WHERE l.reader_id=w.reader_id
		) totals ON true
		WHERE w.source_type='legacy' AND (
			w.recharge_coin_balance <> totals.recharge_income-totals.recharge_expense OR
			w.bonus_coin_balance <> totals.bonus_income-totals.bonus_expense OR
			w.total_recharge_coin_income <> totals.recharge_income OR
			w.total_recharge_coin_expense <> totals.recharge_expense OR
			w.total_bonus_coin_income <> totals.bonus_income OR
			w.total_bonus_coin_expense <> totals.bonus_expense
		)
		ORDER BY w.reader_id LIMIT 1`)
	if err != nil {
		return fmt.Errorf("reconcile migrated reader wallets: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var readerID int64
		if err := rows.Scan(&readerID); err != nil {
			return fmt.Errorf("read wallet reconciliation mismatch: %w", err)
		}
		return fmt.Errorf("reader wallet reconciliation mismatch for reader %d", readerID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reconcile migrated reader wallets: %w", err)
	}
	return nil
}
