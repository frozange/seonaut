package config

import (
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// CrawlerConfig stores the configuration for the crawler.
type CrawlerConfig struct {
	Agent string `mapstructure:"agent"`

	// CrawlLimit is the maximum number of pages stored per crawl. Zero keeps the default.
	CrawlLimit int `mapstructure:"crawl_limit"`

	// AllowedPorts restricts the destination ports the crawler may connect to.
	// Empty means 80 and 443.
	AllowedPorts []int `mapstructure:"allowed_ports"`

	// DenyAddresses lists extra IPs or CIDR ranges the crawler must never reach,
	// on top of the private and reserved ranges that are always refused.
	DenyAddresses []string `mapstructure:"deny_addresses"`
}

// HTTPServerConfig stores the configuration for the HTTP server.
type HTTPServerConfig struct {
	Server string `mapstructure:"host"`
	Port   int    `mapstructure:"port"`
	URL    string `mapstructure:"url"`

	// NoIndex keeps the whole instance out of search engines (X-Robots-Tag and robots.txt).
	NoIndex bool `mapstructure:"noindex"`

	// TrustedProxies lists the reverse proxies allowed to report the visitor address
	// through ClientIPHeader. Leave empty when the app is reached directly.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
	ClientIPHeader string   `mapstructure:"client_ip_header"`

	// Session keys, base64 encoded (64 bytes for authentication, 32 for encryption).
	// When empty, random keys are generated and every restart signs users out.
	SessionAuthKey       string `mapstructure:"session_auth_key"`
	SessionEncryptionKey string `mapstructure:"session_encryption_key"`
}

// TurnstileConfig enables Cloudflare Turnstile on the sign up and sign in forms.
type TurnstileConfig struct {
	SiteKey   string `mapstructure:"site_key"`
	SecretKey string `mapstructure:"secret_key"`
}

// MailConfig is the relay used for account e-mails: an SMTP server (Host) or a
// local sendmail binary (Sendmail). When both are empty, e-mail verification is
// disabled and new accounts are active immediately.
type MailConfig struct {
	Sendmail string `mapstructure:"sendmail"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

// LimitsConfig caps what a single visitor or account can do. Zero disables a limit.
type LimitsConfig struct {
	SignupsPerIPPerDay         int  `mapstructure:"signups_per_ip_per_day"`
	SigninAttemptsPerIPPerHour int  `mapstructure:"signin_attempts_per_ip_per_hour"`
	SigninAttemptsPerEmailHour int  `mapstructure:"signin_attempts_per_email_per_hour"`
	VerificationMailsPerDay    int  `mapstructure:"verification_mails_per_email_per_day"`
	CrawlsPerUserPerDay        int  `mapstructure:"crawls_per_user_per_day"`
	MaxConcurrentCrawls        int  `mapstructure:"max_concurrent_crawls"`
	MaxProjectsPerUser         int  `mapstructure:"max_projects_per_user"`
	AllowCustomUserAgent       bool `mapstructure:"allow_custom_user_agent"`
}

// DBConfig stores the configuration for the database store.
type DBConfig struct {
	Server string `mapstructure:"server"`
	Port   int    `mapstructure:"port"`
	User   string `mapstructure:"user"`
	Pass   string `mapstructure:"password"`
	Name   string `mapstructure:"database"`
}

// UIConfig stores the UI settings.
type UIConfig struct {
	Language string `mapstructure:"language"`
	Theme    string `mapstructure:"theme"`
}

// Config stores the configuration for the application.
type Config struct {
	Crawler    *CrawlerConfig    `mapstructure:"crawler"`
	HTTPServer *HTTPServerConfig `mapstructure:"server"`
	DB         *DBConfig         `mapstructure:"database"`
	UIConfig   *UIConfig         `mapstructure:"UI"`
	Turnstile  *TurnstileConfig  `mapstructure:"turnstile"`
	Mail       *MailConfig       `mapstructure:"mail"`
	Limits     *LimitsConfig     `mapstructure:"limits"`
}

// NewConfig loads the configuration from the specified file and path.
func NewConfig(configFile string) (*Config, error) {
	viper.AddConfigPath(filepath.Dir(configFile))
	viper.SetConfigName(filepath.Base(configFile))
	viper.SetConfigType("toml")

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("SEONAUT") // Optional: prefix for env vars
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Defaults keep every section non-nil and preserve upstream behaviour when a
	// section is absent: no limits, no Turnstile, no e-mail verification.
	viper.SetDefault("server.noindex", false)
	viper.SetDefault("server.client_ip_header", "")
	viper.SetDefault("turnstile.site_key", "")
	viper.SetDefault("turnstile.secret_key", "")
	viper.SetDefault("mail.host", "")
	viper.SetDefault("mail.sendmail", "")
	viper.SetDefault("mail.port", 25)
	viper.SetDefault("limits.signups_per_ip_per_day", 0)
	viper.SetDefault("limits.allow_custom_user_agent", true)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config

	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
