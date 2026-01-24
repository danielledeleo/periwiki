package main

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/securecookie"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const configFilename = "config.yaml"

func SetupConfig() *wiki.Config {
	viper.SetDefault("dbfile", "periwiki.db")
	viper.SetDefault("min_password_length", 8)
	viper.SetDefault("cookie_expiry", 86400*7) // a week
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
	InitLogger(
		ParseLogFormat(viper.GetString("log_format")),
		ParseLogLevel(viper.GetString("log_level")),
	)

	var secretBytes []byte
	_, err = os.Stat(".cookiesecret.yaml")

	if err == nil {
		viper.SetConfigFile(".cookiesecret.yaml")
		viper.AddConfigPath(".")
		err = viper.ReadInConfig()
		if err != nil {
			slog.Error("failed to read cookie secret config", "error", err)
			os.Exit(1)
		}
		secretBytes, err = base64.StdEncoding.DecodeString(viper.GetString("cookie_secret"))
		if err != nil {
			slog.Error("failed to decode cookie secret", "error", err)
			os.Exit(1)
		}
	} else {

		file, err := os.Create(".cookiesecret.yaml")
		if err != nil {
			slog.Error("failed to create cookie secret file", "error", err)
			os.Exit(1)
		}
		defer file.Close()

		secretBytes = securecookie.GenerateRandomKey(64)
		if secretBytes == nil {
			slog.Error("failed to generate cookie secret", "error", errors.New("securecookie.GenerateRandomKey returned nil"))
			os.Exit(1)
		}
		secret := base64.StdEncoding.EncodeToString(secretBytes)

		_, err = file.WriteString("cookie_secret: " + secret + "\n")
		if err != nil {
			slog.Error("failed to write cookie secret", "error", err)
			os.Exit(1)
		}
	}

	config := &wiki.Config{
		MinimumPasswordLength: viper.GetInt("min_password_length"),
		DatabaseFile:          viper.GetString("dbfile"),
		CookieSecret:          secretBytes,
		CookieExpiry:          viper.GetInt("cookie_expiry"),
		Host:                  viper.GetString("host"),
		BaseURL:               viper.GetString("base_url"),
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
