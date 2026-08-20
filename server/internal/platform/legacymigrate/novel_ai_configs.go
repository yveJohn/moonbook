package legacymigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var legacyAISecretPattern = regexp.MustCompile(`^MOONBOOK_AI_[A-Z0-9_]+_API_KEY$`)

// NovelAIConfigsStage migrates AI connection metadata and ordered models. API
// key values are read only to determine whether a deployment Secret reference
// is needed; the value never enters PostgreSQL, logs, or checkpoint metadata.
type NovelAIConfigsStage struct{}

func (NovelAIConfigsStage) Name() string { return "novel-ai-configs" }

type legacyAIConfig struct {
	id, failureThreshold, currentModelID, consecutiveFailures, stateVersion int64
	name, baseURL, model, apiKey, remark                                    string
	enabled                                                                 int64
	created, updated                                                        sql.NullTime
}

type legacyAIModel struct {
	id, configID, sortOrder int64
	name                    string
	created, updated        sql.NullTime
}

func (NovelAIConfigsStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	exists, err := sourceTableExists(ctx, source, "novel_ai_config")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_ai_config", "skipped": "table_not_found"}}, nil
	}
	modelTable, err := sourceTableExists(ctx, source, "novel_ai_config_model")
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `SELECT id,COALESCE(config_name,''),COALESCE(base_url,''),COALESCE(model,''),COALESCE(failure_threshold,5),COALESCE(current_model_id,0),COALESCE(consecutive_failures,0),COALESCE(state_version,0),COALESCE(api_key,''),COALESCE(enabled,1),COALESCE(remark,''),create_time,update_time FROM novel_ai_config WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy AI configs: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_ai_config", "modelTable": modelTable, "secretRefCount": 0}}
	for rows.Next() {
		var item legacyAIConfig
		if err := rows.Scan(&item.id, &item.name, &item.baseURL, &item.model, &item.failureThreshold, &item.currentModelID, &item.consecutiveFailures, &item.stateVersion, &item.apiKey, &item.enabled, &item.remark, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:novel_ai_config:" + result.NextCursor
		if code, message := validateLegacyAIConfig(item); code != "" {
			result.Errors = append(result.Errors, aiError("novel_ai_config", result.NextCursor, code, message))
			continue
		}
		if err := checkLegacyKey(ctx, target, "novel_ai_config", item.id, key); err != nil {
			result.Errors = append(result.Errors, aiError("novel_ai_config", result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		secretRef := ""
		if strings.TrimSpace(item.apiKey) != "" {
			secretRef = fmt.Sprintf("MOONBOOK_AI_LEGACY_%d_API_KEY", item.id)
			if !legacyAISecretPattern.MatchString(secretRef) {
				return BatchResult{}, errors.New("generated AI secret reference failed validation")
			}
			result.Metadata["secretRefCount"] = result.Metadata["secretRefCount"].(int) + 1
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_ai_config(id,config_name,base_url,stream_mode,failure_threshold,current_model_id,consecutive_failures,state_version,secret_env_name,enabled,remark,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,'AUTO',$4,NULL,$5,$6,$7,$8<>0,$9,COALESCE($10,now()),COALESCE($11,now()),$12) ON CONFLICT(id) DO UPDATE SET config_name=EXCLUDED.config_name,base_url=EXCLUDED.base_url,failure_threshold=EXCLUDED.failure_threshold,consecutive_failures=EXCLUDED.consecutive_failures,state_version=EXCLUDED.state_version,secret_env_name=EXCLUDED.secret_env_name,enabled=EXCLUDED.enabled,remark=EXCLUDED.remark,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, strings.TrimSpace(item.name), strings.TrimSpace(item.baseURL), item.failureThreshold, item.consecutiveFailures, item.stateVersion, secretRef, item.enabled, strings.TrimSpace(item.remark), item.created, item.updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert legacy AI config %d: %w", item.id, err)
		}
		models, err := migrateAIModels(ctx, source, target, item, modelTable, &result)
		if err != nil {
			return BatchResult{}, err
		}
		currentModelID := item.currentModelID
		if currentModelID > 0 && !models[currentModelID] {
			result.Errors = append(result.Errors, aiError("novel_ai_config", result.NextCursor, "CURRENT_MODEL_NOT_FOUND", "legacy current model was not found; first migrated model will be selected"))
			currentModelID = 0
		}
		if currentModelID == 0 {
			for id := range models {
				if currentModelID == 0 || id < currentModelID {
					currentModelID = id
				}
			}
		}
		if currentModelID > 0 {
			if _, err := target.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2,updated_at=now() WHERE id=$1`, item.id, currentModelID); err != nil {
				return BatchResult{}, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_ai_config"); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_ai_config_model"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func migrateAIModels(ctx context.Context, source *sql.DB, target *sql.Tx, config legacyAIConfig, modelTable bool, result *BatchResult) (map[int64]bool, error) {
	models := map[int64]bool{}
	if modelTable {
		rows, err := source.QueryContext(ctx, `SELECT id,ai_config_id,COALESCE(model_name,''),COALESCE(sort_order,0),create_time,update_time FROM novel_ai_config_model WHERE ai_config_id=? ORDER BY sort_order,id`, config.id)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var item legacyAIModel
			if err := rows.Scan(&item.id, &item.configID, &item.name, &item.sortOrder, &item.created, &item.updated); err != nil {
				return nil, err
			}
			if code, message := validateLegacyAIModel(item, config.id); code != "" {
				result.Errors = append(result.Errors, aiError("novel_ai_config_model", strconv.FormatInt(item.id, 10), code, message))
				continue
			}
			key := "moonbook-v1:novel_ai_config_model:" + strconv.FormatInt(item.id, 10)
			if err := checkLegacyKey(ctx, target, "novel_ai_config_model", item.id, key); err != nil {
				result.Errors = append(result.Errors, aiError("novel_ai_config_model", strconv.FormatInt(item.id, 10), "SOURCE_KEY_CONFLICT", err.Error()))
				continue
			}
			if _, err := target.ExecContext(ctx, `INSERT INTO novel_ai_config_model(id,ai_config_id,model_name,sort_order,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,COALESCE($5,now()),COALESCE($6,now()),$7) ON CONFLICT(id) DO UPDATE SET ai_config_id=EXCLUDED.ai_config_id,model_name=EXCLUDED.model_name,sort_order=EXCLUDED.sort_order,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, config.id, strings.TrimSpace(item.name), item.sortOrder, item.created, item.updated, key); err != nil {
				return nil, err
			}
			models[item.id] = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	if len(models) == 0 && strings.TrimSpace(config.model) != "" {
		var generatedID int64
		if err := target.QueryRowContext(ctx, `INSERT INTO novel_ai_config_model(ai_config_id,model_name,sort_order) VALUES($1,$2,1) ON CONFLICT(ai_config_id,model_name) DO UPDATE SET updated_at=now() RETURNING id`, config.id, strings.TrimSpace(config.model)).Scan(&generatedID); err != nil {
			return nil, err
		}
		models[generatedID] = true
	}
	return models, nil
}

func validateLegacyAIConfig(item legacyAIConfig) (string, string) {
	if item.id <= 0 || strings.TrimSpace(item.name) == "" || len([]rune(item.name)) > 100 || strings.TrimSpace(item.baseURL) == "" || item.failureThreshold < 1 || item.failureThreshold > 1000 || item.consecutiveFailures < 0 || item.stateVersion < 0 || (item.enabled != 0 && item.enabled != 1) {
		return "INVALID_AI_CONFIG", "legacy AI config fields are invalid"
	}
	return "", ""
}
func validateLegacyAIModel(item legacyAIModel, configID int64) (string, string) {
	if item.id <= 0 || item.configID != configID || strings.TrimSpace(item.name) == "" || len([]rune(item.name)) > 100 || item.sortOrder <= 0 {
		return "INVALID_AI_MODEL", "legacy AI model fields are invalid"
	}
	return "", ""
}
func aiError(table, id, code, message string) RecordError {
	return RecordError{SourceTable: table, SourceID: id, Code: code, Message: message, Retryable: false}
}
