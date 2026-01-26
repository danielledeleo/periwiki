package wiki

// Config holds the configuration for the wiki.
type Config struct {
	CookieSecret              []byte `yaml:"-"`
	CookieExpiry              int    `yaml:"cookie_expiry"`
	DatabaseFile              string `yaml:"dbfile"`
	MinimumPasswordLength     int    `yaml:"minimum_password_length"`
	Host                      string `yaml:"host"`
	BaseURL                   string `yaml:"base_url"`
	AllowAnonymousEditsGlobal bool   `yaml:"allow_anonymous_edits_global"`
	RenderWorkers             int    `yaml:"render_workers"`
}
