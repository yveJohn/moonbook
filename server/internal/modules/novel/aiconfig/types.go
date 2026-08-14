package aiconfig

type ModelInput struct {
	ModelName string `json:"modelName"`
}

type Input struct {
	ConfigName       string       `json:"configName"`
	BaseURL          string       `json:"baseUrl"`
	StreamMode       string       `json:"streamMode"`
	Models           []ModelInput `json:"models"`
	FailureThreshold string       `json:"failureThreshold"`
	SecretEnvName    string       `json:"secretEnvName"`
	Enabled          bool         `json:"enabled"`
	Remark           string       `json:"remark"`
}

type Model struct {
	ID        string `json:"id"`
	ModelName string `json:"modelName"`
	SortOrder int    `json:"sortOrder"`
}

type Config struct {
	ID                  string  `json:"id"`
	ConfigName          string  `json:"configName"`
	BaseURL             string  `json:"baseUrl"`
	RequestMethod       string  `json:"requestMethod"`
	StreamMode          string  `json:"streamMode"`
	Models              []Model `json:"models"`
	ModelCount          int     `json:"modelCount"`
	CurrentModelID      string  `json:"currentModelId"`
	CurrentModelName    string  `json:"currentModelName"`
	FailureThreshold    int     `json:"failureThreshold"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	StateVersion        string  `json:"stateVersion"`
	SecretEnvName       string  `json:"secretEnvName"`
	SecretConfigured    bool    `json:"secretConfigured"`
	Enabled             bool    `json:"enabled"`
	Remark              string  `json:"remark"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
}

type Page struct {
	Items    []Config `json:"list"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

type RuntimeConfig struct {
	ID               int64
	BaseURL          string
	StreamMode       string
	ModelID          int64
	ModelName        string
	FailureThreshold int
	StateVersion     int64
	apiKey           string
}

func (config RuntimeConfig) Snapshot() RuntimeSnapshot {
	return RuntimeSnapshot{ConfigID: config.ID, ModelID: config.ModelID, StateVersion: config.StateVersion}
}

func (config RuntimeConfig) APIKey() string { return config.apiKey }

func (config RuntimeConfig) String() string { return "AI runtime config [secret redacted]" }

func (config RuntimeConfig) GoString() string { return config.String() }

type RuntimeSnapshot struct {
	ConfigID     int64
	ModelID      int64
	StateVersion int64
}
