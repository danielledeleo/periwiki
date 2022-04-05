package main

import (
	"encoding/base64"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/jagger27/periwiki/model"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const configFilename = "config.yaml"

func SetupConfig() *model.Config {
	viper.SetDefault("dbfile", "periwiki.db")
	viper.SetDefault("min_password_length", 8)
	viper.SetDefault("cookie_expiry", 86400*7) // a week
	viper.SetDefault("host", "0.0.0.0:8080")

	viper.SetConfigFile(configFilename)
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	createDefaultConfigFile := false

	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			createDefaultConfigFile = true
		} else {
			log.Fatal(err)
		}
	}

	var secretBytes []byte
	_, err = os.Stat(".cookiesecret.yaml")

	if err == nil {
		viper.SetConfigFile(".cookiesecret.yaml")
		viper.AddConfigPath(".")
		err = viper.ReadInConfig()
		if err != nil {
			log.Fatal(err)
		}
		secretBytes, err = base64.StdEncoding.DecodeString(viper.GetString("cookie_secret"))
		if err != nil {
			log.Fatal(err)
		}
	} else {

		file, err := os.Create(".cookiesecret.yaml")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		secretBytes = securecookie.GenerateRandomKey(64)
		if secretBytes == nil {
			log.Fatal(errors.New("securecookie.GenerateRandomKey returned nil"))
		}
		secret := base64.StdEncoding.EncodeToString(secretBytes)

		_, err = file.WriteString("cookie_secret: " + secret + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}

	config := &model.Config{
		MinimumPasswordLength: viper.GetInt("min_password_length"),
		DatabaseFile:          viper.GetString("dbfile"),
		CookieSecret:          secretBytes,
		CookieExpiry:          viper.GetInt("cookie_expiry"),
		Host:                  viper.GetString("host"),
	}

	if createDefaultConfigFile {
		log.Println("Config not found. Writing defaults to:", configFilename)
		conf, err := os.Create(configFilename)
		if err != nil {
			log.Fatal(err)
		}

		if err := yaml.NewEncoder(conf).Encode(config); err != nil {
			log.Fatal(err)
		}
	}

	return config
}
