package config

import (
	"github.com/spf13/viper"
)

// Config holds application configuration
type Config struct {
	Verbose        bool   `mapstructure:"verbose"`
	DefaultTimeout int    `mapstructure:"default_timeout"`
	DefaultPort    int    `mapstructure:"default_port"`
	LogLevel       string `mapstructure:"log_level"`
}

// Load loads configuration from various sources
func Load() (*Config, error) {
	// Set defaults
	viper.SetDefault("verbose", false)
	viper.SetDefault("default_timeout", 5)
	viper.SetDefault("default_port", 80)
	viper.SetDefault("log_level", "info")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
