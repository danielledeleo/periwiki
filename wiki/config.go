package wiki

// Config holds the file-based configuration for the wiki.
// These are bootstrap settings loaded from config.yaml that are needed
// before the database connection is established.
type Config struct {
	DatabaseFile string `yaml:"dbfile"`
	Host         string `yaml:"host"`
	BaseURL      string `yaml:"base_url"`
	LogFormat    string `yaml:"log_format"`
	LogLevel     string `yaml:"log_level"`
}
