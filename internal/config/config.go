package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/danielledeleo/periwiki/internal/logger"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const configFilename = "config.yaml"

// SetupConfig loads file-based configuration needed for bootstrap.
// Runtime configuration (cookie secret, etc.) is loaded from the database
// after the database connection is established.
func SetupConfig() *wiki.Config {
	viper.SetDefault("dbfile", "periwiki.db")
	viper.SetDefault("host", "0.0.0.0:8080")
	viper.SetDefault("log_format", "pretty") // pretty, json, or text
	viper.SetDefault("log_level", "info")    // debug, info, warn, error
	viper.SetDefault("base_url", "http://localhost:8080")

	viper.SetConfigFile(configFilename)
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	createDefaultConfigFile := false

	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			createDefaultConfigFile = true
		} else {
			slog.Error("failed to read config", "error", err)
			os.Exit(1)
		}
	}

	// Initialize logger with configured format and level
	logger.InitLogger(
		logger.ParseLogFormat(viper.GetString("log_format")),
		logger.ParseLogLevel(viper.GetString("log_level")),
	)

	config := &wiki.Config{
		DatabaseFile: viper.GetString("dbfile"),
		Host:         viper.GetString("host"),
		BaseURL:      viper.GetString("base_url"),
		LogFormat:    viper.GetString("log_format"),
		LogLevel:     viper.GetString("log_level"),
	}

	if createDefaultConfigFile {
		slog.Info("config not found, writing defaults", "file", configFilename)
		conf, err := os.Create(configFilename)
		if err != nil {
			slog.Error("failed to create config file", "error", err)
			os.Exit(1)
		}

		if err := yaml.NewEncoder(conf).Encode(config); err != nil {
			slog.Error("failed to write config file", "error", err)
			os.Exit(1)
		}
	}

	return config
}
