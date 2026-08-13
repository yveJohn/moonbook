package config

type Metrics struct {
	Enabled bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Token   string `mapstructure:"token" json:"-" yaml:"token"`
}
