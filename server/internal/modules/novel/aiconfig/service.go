package aiconfig

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxPageSize = 100

var secretEnvPattern = regexp.MustCompile(`^MOONBOOK_AI_[A-Z0-9_]+_API_KEY$`)

type LookupEnv func(string) (string, bool)

type Service struct {
	db        *sql.DB
	lookupEnv LookupEnv
}

type storedConfig struct {
	id                  int64
	configName          string
	baseURL             string
	streamMode          string
	failureThreshold    int
	currentModelID      int64
	currentModelName    string
	consecutiveFailures int
	stateVersion        int64
	secretEnvName       string
	enabled             bool
	remark              string
	createdAt           time.Time
	updatedAt           time.Time
}

type normalizedInput struct {
	Input
	failureThreshold int
	modelNames       []string
}

func NewService(db *sql.DB, lookup LookupEnv) *Service {
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}
	return &Service{db: db, lookupEnv: lookup}
}

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}

func normalize(input Input) (normalizedInput, error) {
	input.ConfigName = strings.TrimSpace(input.ConfigName)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	input.StreamMode = strings.ToUpper(strings.TrimSpace(input.StreamMode))
	input.FailureThreshold = strings.TrimSpace(input.FailureThreshold)
	input.SecretEnvName = strings.TrimSpace(input.SecretEnvName)
	input.Remark = strings.TrimSpace(input.Remark)
	if input.ConfigName == "" || len([]rune(input.ConfigName)) > 100 {
		return normalizedInput{}, invalid("AI 配置名称不能为空且不能超过 100 个字符")
	}
	parsedURL, err := url.Parse(input.BaseURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || len(input.BaseURL) > 2048 {
		return normalizedInput{}, invalid("基础地址必须是无认证、查询和片段的 HTTP/HTTPS 地址")
	}
	if input.StreamMode == "" {
		input.StreamMode = "AUTO"
	}
	if input.StreamMode != "AUTO" && input.StreamMode != "STREAM" && input.StreamMode != "NON_STREAM" {
		return normalizedInput{}, invalid("调用模式无效")
	}
	threshold, err := strconv.Atoi(input.FailureThreshold)
	if err != nil || threshold < 1 || threshold > 1000 {
		return normalizedInput{}, invalid("失败阈值必须是 1 到 1000 的整数字符串")
	}
	if input.SecretEnvName != "" && !secretEnvPattern.MatchString(input.SecretEnvName) {
		return normalizedInput{}, invalid("Secret 环境变量名必须符合 MOONBOOK_AI_*_API_KEY")
	}
	if len([]rune(input.Remark)) > 500 {
		return normalizedInput{}, invalid("备注不能超过 500 个字符")
	}
	if len(input.Models) == 0 || len(input.Models) > 50 {
		return normalizedInput{}, invalid("AI 配置需要 1 到 50 个模型")
	}
	modelNames := make([]string, 0, len(input.Models))
	seen := make(map[string]struct{}, len(input.Models))
	for _, model := range input.Models {
		name := strings.TrimSpace(model.ModelName)
		if name == "" || len([]rune(name)) > 100 {
			return normalizedInput{}, invalid("模型名称不能为空且不能超过 100 个字符")
		}
		if _, exists := seen[name]; exists {
			return normalizedInput{}, invalid("同一 AI 配置的模型名称不能重复")
		}
		seen[name] = struct{}{}
		modelNames = append(modelNames, name)
	}
	return normalizedInput{Input: input, failureThreshold: threshold, modelNames: modelNames}, nil
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, invalid("AI 配置 ID 必须是正整数字符串")
	}
	return id, nil
}

func (s *Service) render(ctx context.Context, stored storedConfig, includeModels bool) (Config, error) {
	result := Config{
		ID: strconv.FormatInt(stored.id, 10), ConfigName: stored.configName, BaseURL: stored.baseURL,
		RequestMethod: "POST", StreamMode: stored.streamMode, CurrentModelID: strconv.FormatInt(stored.currentModelID, 10),
		CurrentModelName: stored.currentModelName, FailureThreshold: stored.failureThreshold,
		ConsecutiveFailures: stored.consecutiveFailures, StateVersion: strconv.FormatInt(stored.stateVersion, 10),
		SecretEnvName: stored.secretEnvName, Enabled: stored.enabled, Remark: stored.remark,
		CreatedAt: stored.createdAt.Format(time.RFC3339Nano), UpdatedAt: stored.updatedAt.Format(time.RFC3339Nano),
	}
	if stored.secretEnvName != "" {
		value, ok := s.lookupEnv(stored.secretEnvName)
		result.SecretConfigured = ok && strings.TrimSpace(value) != ""
	}
	if includeModels {
		models, err := loadModels(ctx, s.db, stored.id)
		if err != nil {
			return Config{}, err
		}
		result.Models = models
		result.ModelCount = len(models)
	}
	return result, nil
}

const selectStored = `SELECT c.id,c.config_name,c.base_url,c.stream_mode,c.failure_threshold,
	c.current_model_id,m.model_name,c.consecutive_failures,c.state_version,c.secret_env_name,
	c.enabled,c.remark,c.created_at,c.updated_at
	FROM novel_ai_config c JOIN novel_ai_config_model m ON m.id=c.current_model_id AND m.ai_config_id=c.id`

func scanStored(scanner interface{ Scan(...any) error }) (storedConfig, error) {
	var config storedConfig
	err := scanner.Scan(&config.id, &config.configName, &config.baseURL, &config.streamMode,
		&config.failureThreshold, &config.currentModelID, &config.currentModelName,
		&config.consecutiveFailures, &config.stateVersion, &config.secretEnvName,
		&config.enabled, &config.remark, &config.createdAt, &config.updatedAt)
	return config, err
}

func loadModels(ctx context.Context, query interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, configID int64) ([]Model, error) {
	rows, err := query.QueryContext(ctx, `SELECT id::text,model_name,sort_order FROM novel_ai_config_model WHERE ai_config_id=$1 ORDER BY sort_order,id`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := []Model{}
	for rows.Next() {
		var model Model
		if err := rows.Scan(&model.ID, &model.ModelName, &model.SortOrder); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func (s *Service) List(ctx context.Context, keyword, enabled string, page, pageSize int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	keyword, enabled = strings.TrimSpace(keyword), strings.TrimSpace(enabled)
	if enabled != "" && enabled != "true" && enabled != "false" {
		return Page{}, invalid("启用状态筛选无效")
	}
	where := ` WHERE c.deleted_at IS NULL
		AND ($1='' OR c.config_name ILIKE '%'||$1||'%' OR c.base_url ILIKE '%'||$1||'%' OR EXISTS(
			SELECT 1 FROM novel_ai_config_model filter_model WHERE filter_model.ai_config_id=c.id AND filter_model.model_name ILIKE '%'||$1||'%'))
		AND ($2='' OR c.enabled::text=$2)`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM novel_ai_config c`+where, keyword, enabled).Scan(&total); err != nil {
		return Page{}, err
	}
	rows, err := s.db.QueryContext(ctx, selectStored+where+` ORDER BY c.id LIMIT $3 OFFSET $4`, keyword, enabled, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := []Config{}
	for rows.Next() {
		stored, err := scanStored(rows)
		if err != nil {
			return Page{}, err
		}
		item, err := s.render(ctx, stored, true)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	return Page{Items: items, Total: total, Page: page, PageSize: pageSize}, rows.Err()
}

func (s *Service) EnabledOptions(ctx context.Context) ([]Config, error) {
	rows, err := s.db.QueryContext(ctx, selectStored+` WHERE c.deleted_at IS NULL AND c.enabled ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Config{}
	for rows.Next() {
		stored, err := scanStored(rows)
		if err != nil {
			return nil, err
		}
		item, err := s.render(ctx, stored, true)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Get(ctx context.Context, rawID string) (Config, error) {
	id, err := parseID(rawID)
	if err != nil {
		return Config{}, err
	}
	stored, err := scanStored(s.db.QueryRowContext(ctx, selectStored+` WHERE c.id=$1 AND c.deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "AI 配置不存在")
	}
	if err != nil {
		return Config{}, err
	}
	return s.render(ctx, stored, true)
}

func insertModels(ctx context.Context, tx *sql.Tx, configID int64, names []string) ([]int64, error) {
	ids := make([]int64, 0, len(names))
	for index, name := range names {
		var id int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO novel_ai_config_model(ai_config_id,model_name,sort_order) VALUES($1,$2,$3) RETURNING id`, configID, name, index+1).Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Service) Create(ctx context.Context, input Input) (Config, error) {
	clean, err := normalize(input)
	if err != nil {
		return Config{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Config{}, err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO novel_ai_config(config_name,base_url,stream_mode,failure_threshold,secret_env_name,enabled,remark) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, clean.ConfigName, clean.BaseURL, clean.StreamMode, clean.failureThreshold, clean.SecretEnvName, clean.Enabled, clean.Remark).Scan(&id); err != nil {
		return Config{}, mapWriteError(err)
	}
	modelIDs, err := insertModels(ctx, tx, id, clean.modelNames)
	if err != nil {
		return Config{}, mapWriteError(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2 WHERE id=$1`, id, modelIDs[0]); err != nil {
		return Config{}, mapWriteError(err)
	}
	if err := tx.Commit(); err != nil {
		return Config{}, mapWriteError(err)
	}
	return s.Get(ctx, strconv.FormatInt(id, 10))
}

func modelNames(models []Model) []string {
	names := make([]string, len(models))
	for index := range models {
		names[index] = models[index].ModelName
	}
	return names
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (s *Service) Update(ctx context.Context, rawID string, input Input) (Config, error) {
	id, err := parseID(rawID)
	if err != nil {
		return Config{}, err
	}
	clean, err := normalize(input)
	if err != nil {
		return Config{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Config{}, err
	}
	defer tx.Rollback()
	var oldThreshold int
	var currentModelID, stateVersion int64
	var failures int
	if err := tx.QueryRowContext(ctx, `SELECT failure_threshold,current_model_id,consecutive_failures,state_version FROM novel_ai_config WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&oldThreshold, &currentModelID, &failures, &stateVersion); errors.Is(err, sql.ErrNoRows) {
		return Config{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "AI 配置不存在")
	} else if err != nil {
		return Config{}, err
	}
	storedModels, err := loadModels(ctx, tx, id)
	if err != nil {
		return Config{}, err
	}
	modelsChanged := !equalStrings(modelNames(storedModels), clean.modelNames)
	if modelsChanged {
		if _, err := tx.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=NULL WHERE id=$1`, id); err != nil {
			return Config{}, mapWriteError(err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM novel_ai_config_model WHERE ai_config_id=$1`, id); err != nil {
			return Config{}, mapWriteError(err)
		}
		ids, err := insertModels(ctx, tx, id, clean.modelNames)
		if err != nil {
			return Config{}, mapWriteError(err)
		}
		currentModelID = ids[0]
	}
	if modelsChanged || oldThreshold != clean.failureThreshold {
		if !modelsChanged {
			parsed, err := strconv.ParseInt(storedModels[0].ID, 10, 64)
			if err != nil {
				return Config{}, err
			}
			currentModelID = parsed
		}
		failures = 0
		stateVersion++
	}
	if _, err := tx.ExecContext(ctx, `UPDATE novel_ai_config SET config_name=$2,base_url=$3,stream_mode=$4,failure_threshold=$5,current_model_id=$6,consecutive_failures=$7,state_version=$8,secret_env_name=$9,enabled=$10,remark=$11,updated_at=now() WHERE id=$1`, id, clean.ConfigName, clean.BaseURL, clean.StreamMode, clean.failureThreshold, currentModelID, failures, stateVersion, clean.SecretEnvName, clean.Enabled, clean.Remark); err != nil {
		return Config{}, mapWriteError(err)
	}
	if err := tx.Commit(); err != nil {
		return Config{}, mapWriteError(err)
	}
	return s.Get(ctx, rawID)
}

func (s *Service) Delete(ctx context.Context, rawID string) error {
	id, err := parseID(rawID)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE novel_ai_config SET enabled=false,deleted_at=now(),updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapWriteError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "AI 配置不存在")
	}
	return nil
}

func (s *Service) ResetModelState(ctx context.Context, rawID string) (Config, error) {
	id, err := parseID(rawID)
	if err != nil {
		return Config{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Config{}, err
	}
	defer tx.Rollback()
	var stateVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT state_version FROM novel_ai_config WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&stateVersion); errors.Is(err, sql.ErrNoRows) {
		return Config{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "AI 配置不存在")
	} else if err != nil {
		return Config{}, err
	}
	var firstModelID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM novel_ai_config_model WHERE ai_config_id=$1 ORDER BY sort_order,id LIMIT 1`, id).Scan(&firstModelID); err != nil {
		return Config{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2,consecutive_failures=0,state_version=$3,updated_at=now() WHERE id=$1`, id, firstModelID, stateVersion+1); err != nil {
		return Config{}, err
	}
	if err := tx.Commit(); err != nil {
		return Config{}, err
	}
	return s.Get(ctx, rawID)
}

func (s *Service) ResolveEnabled(ctx context.Context, rawID string) (RuntimeConfig, error) {
	id, err := parseID(rawID)
	if err != nil {
		return RuntimeConfig{}, err
	}
	stored, err := scanStored(s.db.QueryRowContext(ctx, selectStored+` WHERE c.id=$1 AND c.deleted_at IS NULL AND c.enabled`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return RuntimeConfig{}, invalid("AI 配置不存在或未启用")
	}
	if err != nil {
		return RuntimeConfig{}, err
	}
	apiKey := ""
	if stored.secretEnvName != "" {
		value, ok := s.lookupEnv(stored.secretEnvName)
		if !ok || strings.TrimSpace(value) == "" {
			return RuntimeConfig{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "AI 配置的 Secret 尚未注入")
		}
		apiKey = value
	}
	return RuntimeConfig{ID: stored.id, BaseURL: stored.baseURL, StreamMode: stored.streamMode,
		ModelID: stored.currentModelID, ModelName: stored.currentModelName,
		FailureThreshold: stored.failureThreshold, StateVersion: stored.stateVersion, apiKey: apiKey}, nil
}

func (s *Service) RecordCallSuccess(ctx context.Context, snapshot RuntimeSnapshot) error {
	return s.updateRuntime(ctx, snapshot, true)
}

func (s *Service) RecordCallFailure(ctx context.Context, snapshot RuntimeSnapshot) error {
	return s.updateRuntime(ctx, snapshot, false)
}

func (s *Service) updateRuntime(ctx context.Context, snapshot RuntimeSnapshot, success bool) error {
	if snapshot.ConfigID <= 0 || snapshot.ModelID <= 0 || snapshot.StateVersion < 0 {
		return invalid("AI 调用快照无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var currentModelID, stateVersion int64
	var failures, threshold int
	if err := tx.QueryRowContext(ctx, `SELECT current_model_id,state_version,consecutive_failures,failure_threshold FROM novel_ai_config WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, snapshot.ConfigID).Scan(&currentModelID, &stateVersion, &failures, &threshold); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if currentModelID != snapshot.ModelID || stateVersion != snapshot.StateVersion {
		return nil
	}
	if success {
		if failures == 0 {
			return nil
		}
		_, err = tx.ExecContext(ctx, `UPDATE novel_ai_config SET consecutive_failures=0,updated_at=now() WHERE id=$1`, snapshot.ConfigID)
	} else if failures+1 < threshold {
		_, err = tx.ExecContext(ctx, `UPDATE novel_ai_config SET consecutive_failures=$2,updated_at=now() WHERE id=$1`, snapshot.ConfigID, failures+1)
	} else {
		var nextModelID int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM novel_ai_config_model WHERE ai_config_id=$1 AND sort_order>(SELECT sort_order FROM novel_ai_config_model WHERE id=$2 AND ai_config_id=$1) ORDER BY sort_order,id LIMIT 1`, snapshot.ConfigID, currentModelID).Scan(&nextModelID)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `SELECT id FROM novel_ai_config_model WHERE ai_config_id=$1 ORDER BY sort_order,id LIMIT 1`, snapshot.ConfigID).Scan(&nextModelID)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2,consecutive_failures=0,state_version=$3,updated_at=now() WHERE id=$1`, snapshot.ConfigID, nextModelID, stateVersion+1)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, "AI 配置模型名称或顺序重复")
		case "23503", "23514":
			return apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "AI 配置关联或字段无效")
		}
	}
	return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存 AI 配置失败")
}
