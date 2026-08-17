package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/adhocore/gronx"
	"github.com/spf13/viper"
)

// EnvPrefix prefixes every env override: GOTH_TEMPLATE_SERVER_PORT, GOTH_TEMPLATE_DATABASE_URL, …
const EnvPrefix = "GOTH_TEMPLATE"

// ScheduleTZ anchors every cron schedule whatever the host's zone.
const ScheduleTZ = "UTC"

// minScheduleGap floors cron schedule density; scheduleGapHorizon bounds the walk.
const (
	minScheduleGap     = 1 * time.Hour
	scheduleGapHorizon = 400 * 24 * time.Hour
)

// Config mirrors the YAML file. Every field is overridable by env
// ({EnvPrefix}_{SECTION}_{KEY}); see config.example.yaml.
type Config struct {
	Server struct {
		Name string `mapstructure:"name"`
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
		// BaseURL is the browser-reachable origin (scheme://host[:port]); the CSRF
		// origin check compares against it.
		BaseURL       string `mapstructure:"base_url"`
		Timeout       int    `mapstructure:"timeout"`
		LogLevel      string `mapstructure:"log_level"`
		Environment   string `mapstructure:"environment"`
		CSPReportOnly bool   `mapstructure:"csp_report_only"`
		DrainGraceSec int    `mapstructure:"drain_grace_sec"`
		// SyncIntervalMin drives the demo PeriodicRunner; SyncAt (cron, empty
		// disables) drives the demo ScheduledRunner.
		SyncIntervalMin int    `mapstructure:"sync_interval_min"`
		SyncAt          string `mapstructure:"sync_at"`
	} `mapstructure:"server"`

	Database struct {
		Engine         string `mapstructure:"engine"`
		URL            string `mapstructure:"url"`
		SkipMigrations bool   `mapstructure:"skip_migrations"`
		PoolMaxConns   int    `mapstructure:"pool_max_conns"`
	} `mapstructure:"database"`

	// Redis backs the session store and the cache wrappers in utils. Empty URL
	// disables both — every consumer degrades to "skip cache, hit source".
	Redis struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"redis"`

	Session struct {
		MaxAge int  `mapstructure:"max_age"`
		Secure bool `mapstructure:"secure"`
	} `mapstructure:"session"`

	// Precedence: bare OTEL_* env (standard, wins) → {EnvPrefix}_OTLP_* env → file
	// → defaults. Empty url and no env = tracing off.
	Otlp struct {
		Url      string `mapstructure:"url"`
		Protocol string `mapstructure:"protocol"` // grpc | http
		Secure   bool   `mapstructure:"secure"`
	} `mapstructure:"otlp"`

	Metrics struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"metrics"`
}

// LoadConfig reads and validates configuration from the default file locations.
func LoadConfig() (*Config, error) {
	v := viper.New()
	applyDefaults(v)
	applyEnv(v)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return parseAndValidate(v)
}

func applyDefaults(v *viper.Viper) {
	v.SetDefault("server.name", "goth-template")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.timeout", 10)
	v.SetDefault("server.base_url", "http://localhost:8080")
	v.SetDefault("server.log_level", "INFO")
	v.SetDefault("server.environment", "development")
	v.SetDefault("server.csp_report_only", false)
	v.SetDefault("server.drain_grace_sec", 5)
	v.SetDefault("server.sync_interval_min", 60)
	v.SetDefault("server.sync_at", "")
	v.SetDefault("database.engine", "postgres")
	v.SetDefault("session.max_age", 3600)
	v.SetDefault("session.secure", true)
	v.SetDefault("otlp.protocol", "grpc")
	v.SetDefault("otlp.secure", false)
	v.SetDefault("metrics.enabled", false)
}

func applyEnv(v *viper.Viper) {
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func parseAndValidate(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Sanitize(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate is used to check configuration values
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.Timeout < 1 {
		return fmt.Errorf("server.timeout must be at least 1")
	}
	if !strings.HasPrefix(c.Server.BaseURL, "http://") && !strings.HasPrefix(c.Server.BaseURL, "https://") {
		return fmt.Errorf("server.base_url must start with http:// or https://, got %q", c.Server.BaseURL)
	}
	if c.Server.DrainGraceSec < 0 {
		return fmt.Errorf("server.drain_grace_sec cannot be negative (0 disables the drain wait)")
	}
	if c.Server.SyncIntervalMin < 1 {
		// Feeds time.NewTicker, which panics on a non-positive interval.
		return fmt.Errorf("server.sync_interval_min must be at least 1")
	}
	// Empty disables the scheduled pass.
	if c.Server.SyncAt != "" {
		if !gronx.IsValid(c.Server.SyncAt) {
			return fmt.Errorf("server.sync_at must be a valid cron expression, got %q", c.Server.SyncAt)
		}
		if err := validateScheduleGap(c.Server.SyncAt, minScheduleGap); err != nil {
			return fmt.Errorf("server.sync_at: %w", err)
		}
	}

	if strings.ToLower(c.Database.Engine) != "postgres" {
		return fmt.Errorf("database.engine must be postgres")
	}
	if c.Database.URL == "" {
		return fmt.Errorf("database.url is required")
	}
	if c.Database.PoolMaxConns < 0 {
		return fmt.Errorf("database.pool_max_conns cannot be negative (0 = take it from the DSN)")
	}

	if c.Session.MaxAge < 0 {
		return fmt.Errorf("session.max_age must be positive")
	}

	switch c.Otlp.Protocol {
	case "grpc", "http":
	default:
		return fmt.Errorf("otlp.protocol must be grpc or http, got %q", c.Otlp.Protocol)
	}

	return nil
}

// Sanitize normalizes values Validate then checks.
func (c *Config) Sanitize() error {
	c.Otlp.Url = strings.TrimPrefix(c.Otlp.Url, "https://")
	c.Otlp.Url = strings.TrimPrefix(c.Otlp.Url, "http://")
	return nil
}

// validateScheduleGap walks the spec's fires across the horizon and rejects any
// consecutive gap under floor; specs gronx cannot advance (never fire) fail too.
// Fires are computed in ScheduleTZ to match the runner.
func validateScheduleGap(spec string, floor time.Duration) error {
	loc, err := time.LoadLocation(ScheduleTZ)
	if err != nil {
		return fmt.Errorf("cannot load schedule timezone %s: %w", ScheduleTZ, err)
	}
	prev, err := gronx.NextTickAfter(spec, time.Now().In(loc), false)
	if err != nil {
		return fmt.Errorf("cannot compute fire times: %w", err)
	}
	limit := prev.Add(scheduleGapHorizon)
	for prev.Before(limit) {
		next, err := gronx.NextTickAfter(spec, prev, false)
		if err != nil {
			return fmt.Errorf("cannot compute fire times: %w", err)
		}
		if gap := next.Sub(prev); gap < floor && wallGap(prev, next, gap) < floor {
			return fmt.Errorf("fires %s apart around %s; minimum gap is %s", gap, next.Format(time.RFC3339), floor)
		}
		prev = next
	}
	return nil
}

// wallGap widens an elapsed gap back to its wall-clock width across a DST shift.
func wallGap(prev, next time.Time, gap time.Duration) time.Duration {
	_, prevOff := prev.Zone()
	_, nextOff := next.Zone()
	return gap + time.Duration(nextOff-prevOff)*time.Second
}
