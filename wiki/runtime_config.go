package wiki

import (
	"database/sql"
	"encoding/base64"
	"log/slog"
	"strconv"

	"github.com/gorilla/securecookie"
)

// RuntimeConfig holds configuration values stored in the database.
// These settings can be modified at runtime without restarting the application.
type RuntimeConfig struct {
	CookieSecret              []byte
	CookieExpiry              int
	MinimumPasswordLength     int
	AllowAnonymousEditsGlobal bool
	AllowSignups              bool
	RenderWorkers             int
}

// Setting key constants
const (
	SettingCookieSecret              = "cookie_secret"
	SettingAllowAnonymousEditsGlobal = "allow_anonymous_edits_global"
	SettingAllowSignups              = "allow_signups"
	SettingRenderWorkers             = "render_workers"
	SettingCookieExpiry              = "cookie_expiry"
	SettingMinPasswordLength         = "min_password_length"
	SettingRenderTemplateHash        = "render_template_hash"
	SettingSetupVersion              = "setup_version"
	SettingSchemaVersion             = "schema_version"
)

// Default values for runtime settings
const (
	DefaultCookieExpiry          = 604800 // 7 days
	DefaultMinPasswordLength     = 8
	DefaultAllowAnonymousEdits   = false
	DefaultAllowSignups          = true
	DefaultRenderWorkers         = 0 // 0 = auto-detect
)

// LoadRuntimeConfig loads runtime configuration from the database.
// If settings don't exist, it creates them with default values.
func LoadRuntimeConfig(db *sql.DB) (*RuntimeConfig, error) {
	config := &RuntimeConfig{}

	// Load or create cookie_secret
	cookieSecretB64, err := GetOrCreateSetting(db, SettingCookieSecret, func() string {
		secret := securecookie.GenerateRandomKey(64)
		if secret == nil {
			slog.Error("failed to generate cookie secret")
			return ""
		}
		return base64.StdEncoding.EncodeToString(secret)
	})
	if err != nil {
		return nil, err
	}
	config.CookieSecret, err = base64.StdEncoding.DecodeString(cookieSecretB64)
	if err != nil {
		return nil, err
	}

	// Load or create allow_anonymous_edits_global
	allowAnonStr, err := GetOrCreateSetting(db, SettingAllowAnonymousEditsGlobal, func() string {
		return strconv.FormatBool(DefaultAllowAnonymousEdits)
	})
	if err != nil {
		return nil, err
	}
	config.AllowAnonymousEditsGlobal, err = strconv.ParseBool(allowAnonStr)
	if err != nil {
		return nil, err
	}

	// Load or create allow_signups
	allowSignupsStr, err := GetOrCreateSetting(db, SettingAllowSignups, func() string {
		return strconv.FormatBool(DefaultAllowSignups)
	})
	if err != nil {
		return nil, err
	}
	config.AllowSignups, err = strconv.ParseBool(allowSignupsStr)
	if err != nil {
		return nil, err
	}

	// Load or create render_workers
	renderWorkersStr, err := GetOrCreateSetting(db, SettingRenderWorkers, func() string {
		return strconv.Itoa(DefaultRenderWorkers)
	})
	if err != nil {
		return nil, err
	}
	config.RenderWorkers, err = strconv.Atoi(renderWorkersStr)
	if err != nil {
		return nil, err
	}

	// Load or create cookie_expiry
	cookieExpiryStr, err := GetOrCreateSetting(db, SettingCookieExpiry, func() string {
		return strconv.Itoa(DefaultCookieExpiry)
	})
	if err != nil {
		return nil, err
	}
	config.CookieExpiry, err = strconv.Atoi(cookieExpiryStr)
	if err != nil {
		return nil, err
	}

	// Load or create min_password_length
	minPwLengthStr, err := GetOrCreateSetting(db, SettingMinPasswordLength, func() string {
		return strconv.Itoa(DefaultMinPasswordLength)
	})
	if err != nil {
		return nil, err
	}
	config.MinimumPasswordLength, err = strconv.Atoi(minPwLengthStr)
	if err != nil {
		return nil, err
	}

	slog.Info("runtime config loaded from database")
	return config, nil
}

// GetOrCreateSetting retrieves a setting from the database, or creates it with
// the value returned by defaultFn if it doesn't exist.
func GetOrCreateSetting(db *sql.DB, key string, defaultFn func() string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM Setting WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		// Setting doesn't exist, create it
		value = defaultFn()
		_, err = db.Exec(
			"INSERT INTO Setting (key, value) VALUES (?, ?)",
			key, value,
		)
		if err != nil {
			return "", err
		}
		slog.Info("created default setting", "key", key)
		return value, nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// UpdateSetting updates an existing setting or creates it if it doesn't exist.
func UpdateSetting(db *sql.DB, key string, value string) error {
	result, err := db.Exec(
		"UPDATE Setting SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE key = ?",
		value, key,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		_, err = db.Exec(
			"INSERT INTO Setting (key, value) VALUES (?, ?)",
			key, value,
		)
		return err
	}
	return nil
}
