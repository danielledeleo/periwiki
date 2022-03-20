package main

import (
	"encoding/base64"
	"errors"
	"log"
	"os"

	"github.com/gorilla/securecookie"
	"github.com/jagger27/iwikii/model"
	"github.com/spf13/viper"
)

func SetupConfig() *model.Config {
	viper.SetDefault("dbfile", "iwikii.db")
	viper.SetDefault("min_password_length", 8)
	viper.SetDefault("cookie_expiry", 86400*7) // a week
	viper.SetDefault("host", "")
	viper.SetDefault("port", "8080")

	viper.SetConfigFile("config.yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		log.Fatal(err)
	}
	_, err = os.Stat(".cookiesecret.yaml")

	var secretBytes []byte
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
		defer file.Close()
		if err != nil {
			log.Fatal(err)
		}

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

	return &model.Config{
		MinimumPasswordLength: viper.GetInt("min_password_length"),
		DatabaseFile:          viper.GetString("dbfile"),
		CookieSecret:          secretBytes,
		CookieExpiry:          viper.GetInt("cookie_expiry"),
	}
}
