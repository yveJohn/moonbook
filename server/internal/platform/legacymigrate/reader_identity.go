package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ReaderIdentityStage migrates reader accounts and invite facts. Passwords are
// copied as hashes only; migration diagnostics never include their values.
type ReaderIdentityStage struct{}

func (ReaderIdentityStage) Name() string { return "reader-identity" }

var md5Hash = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
var bcryptHash = regexp.MustCompile(`^\$2[aby]?\$[0-9]{2}\$[./A-Za-z0-9]{53}$`)

func passwordAlgorithm(hash string) (string, bool) {
	hash = strings.TrimSpace(hash)
	if bcryptHash.MatchString(hash) {
		return "bcrypt", true
	}
	if md5Hash.MatchString(hash) {
		return "md5", true
	}
	return "", false
}

func (ReaderIdentityStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, last, err := stageCursor(cursor, "reader_user")
	if err != nil {
		return BatchResult{}, err
	}
	if table == "reader_user" {
		exists, err := sourceTableExists(ctx, source, "reader_user")
		if err != nil {
			return BatchResult{}, err
		}
		if exists {
			return migrateReaderUsers(ctx, source, target, last, limit)
		}
		table, last = "user", 0
	}
	if table == "user" {
		exists, err := sourceTableExists(ctx, source, "user")
		if err != nil {
			return BatchResult{}, err
		}
		if exists {
			return migrateLegacyUsers(ctx, source, target, last, limit)
		}
	}
	return BatchResult{NextCursor: "user:0", Done: true, Metadata: map[string]any{"source": "reader_user,user", "skipped": "table_not_found"}}, nil
}

func stageCursor(raw, first string) (string, int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return first, 0, nil
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid stage cursor %q", raw)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id < 0 {
		return "", 0, fmt.Errorf("invalid stage cursor %q", raw)
	}
	return parts[0], id, nil
}

func migrateReaderUsers(ctx context.Context, source *sql.DB, target *sql.Tx, last int64, limit int) (BatchResult, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,username,COALESCE(nickname,''),password_hash,COALESCE(status,'enabled'),COALESCE(invite_code_id,0),last_login_time,create_time,update_time FROM reader_user WHERE id>? ORDER BY id LIMIT ?`, last, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: fmt.Sprintf("reader_user:%d", last), Metadata: map[string]any{"source": "reader_user"}}
	for rows.Next() {
		var id, invite int64
		var username, nick, hash, status string
		var login, created, updated sql.NullTime
		if err := rows.Scan(&id, &username, &nick, &hash, &status, &invite, &login, &created, &updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = fmt.Sprintf("reader_user:%d", id)
		alg, ok := passwordAlgorithm(hash)
		if !ok {
			result.Errors = append(result.Errors, RecordError{SourceTable: "reader_user", SourceID: strconv.FormatInt(id, 10), Code: "INVALID_PASSWORD_HASH", Message: "legacy password hash format is unsupported"})
			continue
		}
		if _, err := target.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,password_algorithm,status,invite_code_id,last_login_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,0),$8,COALESCE($9,now()),COALESCE($10,now())) ON CONFLICT(id) DO UPDATE SET username=EXCLUDED.username,nickname=EXCLUDED.nickname,password_hash=EXCLUDED.password_hash,password_algorithm=EXCLUDED.password_algorithm,status=EXCLUDED.status,invite_code_id=EXCLUDED.invite_code_id,last_login_at=EXCLUDED.last_login_at,updated_at=EXCLUDED.updated_at`, id, strings.TrimSpace(username), strings.TrimSpace(nick), hash, alg, status, invite, nullableLegacyTime(login), nullableLegacyTime(created), nullableLegacyTime(updated)); err != nil {
			return BatchResult{}, err
		}
	}
	result.Done = result.Processed < int64(limit)
	if result.Done {
		result.Done = false
		result.NextCursor = "user:0"
	}
	return result, nil
}

func migrateLegacyUsers(ctx context.Context, source *sql.DB, target *sql.Tx, last int64, limit int) (BatchResult, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,username,password,COALESCE(nick_name,''),COALESCE(status,0),create_time,update_time FROM user WHERE id>? ORDER BY id LIMIT ?`, last, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: fmt.Sprintf("user:%d", last), Metadata: map[string]any{"source": "user"}}
	for rows.Next() {
		var id int64
		var username, hash, nick string
		var status int
		var created, updated sql.NullTime
		if err := rows.Scan(&id, &username, &hash, &nick, &status, &created, &updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = fmt.Sprintf("user:%d", id)
		alg, ok := passwordAlgorithm(hash)
		if !ok {
			result.Errors = append(result.Errors, RecordError{SourceTable: "user", SourceID: strconv.FormatInt(id, 10), Code: "INVALID_PASSWORD_HASH", Message: "legacy password hash format is unsupported"})
			continue
		}
		_, err := target.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,password_algorithm,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,COALESCE($7,now()),COALESCE($8,now())) ON CONFLICT(id) DO NOTHING`, id, strings.TrimSpace(username), strings.TrimSpace(nick), hash, alg, mapLegacyUserStatus(status), nullableLegacyTime(created), nullableLegacyTime(updated))
		if err != nil {
			return BatchResult{}, err
		}
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func mapLegacyUserStatus(status int) string {
	if status == 0 {
		return "enabled"
	}
	return "disabled"
}
