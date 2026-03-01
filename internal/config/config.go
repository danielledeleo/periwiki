package config

import (
	"log/slog"
	"os"

	"github.com/danielledeleo/periwiki/internal/logger"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/spf13/viper"
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
	if err := viper.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			slog.Error("failed to read config", "error", err)
			os.Exit(1)
		}
	}

	// Initialize logger with configured format and level
	logger.InitLogger(
		logger.ParseLogFormat(viper.GetString("log_format")),
		logger.ParseLogLevel(viper.GetString("log_level")),
	)

	return &wiki.Config{
		DatabaseFile: viper.GetString("dbfile"),
		Host:         viper.GetString("host"),
		BaseURL:      viper.GetString("base_url"),
		LogFormat:    viper.GetString("log_format"),
		LogLevel:     viper.GetString("log_level"),
	}
}
