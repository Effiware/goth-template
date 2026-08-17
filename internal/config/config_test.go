package config

import (
	"bytes"
	_ "embed"
	"io"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/valid.yaml
var validYAML string

//go:embed testdata/minimal.yaml
var minimalYAML string

// loadConfigFromReader composes the same private helpers as LoadConfig, minus the
// file lookup.
func loadConfigFromReader(r io.Reader) (*Config, error) {
	v := viper.New()
	applyDefaults(v)
	applyEnv(v)
	v.SetConfigType("yaml")
	if err := v.ReadConfig(r); err != nil {
		return nil, err
	}
	return parseAndValidate(v)
}

func mustLoad(t *testing.T, raw string) *Config {
	t.Helper()
	cfg, err := loadConfigFromReader(bytes.NewBufferString(raw))
	require.NoError(t, err)
	return cfg
}

// validBaseline is the smallest Config that passes Validate; tests mutate a copy
// to exercise one condition each.
func validBaseline() *Config {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Server.Timeout = 10
	cfg.Server.BaseURL = "http://localhost:8080"
	cfg.Server.SyncIntervalMin = 60
	cfg.Database.Engine = "postgres"
	cfg.Database.URL = "postgres://user:pass@localhost:5432/goth"
	cfg.Session.MaxAge = 3600
	cfg.Otlp.Protocol = "grpc"
	return cfg
}

func TestLoadValidFile(t *testing.T) {
	cfg := mustLoad(t, validYAML)

	assert.Equal(t, "goth-template-test", cfg.Server.Name)
	assert.Equal(t, 8083, cfg.Server.Port)
	assert.Equal(t, 30, cfg.Server.Timeout)
	assert.Equal(t, "postgres://user:pass@localhost:5432/goth", cfg.Database.URL)
	assert.Equal(t, "redis://localhost:6379", cfg.Redis.URL)
	assert.False(t, cfg.Session.Secure)
	assert.True(t, cfg.Metrics.Enabled)
	// Sanitize strips the scheme — the OTLP exporter wants host:port.
	assert.Equal(t, "otel.example.com:4318", cfg.Otlp.Url)
}

func TestDefaultsFillTheGaps(t *testing.T) {
	cfg := mustLoad(t, minimalYAML)

	assert.Equal(t, "goth-template", cfg.Server.Name)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 10, cfg.Server.Timeout)
	assert.Equal(t, "http://localhost:8080", cfg.Server.BaseURL)
	assert.Equal(t, "INFO", cfg.Server.LogLevel)
	assert.Equal(t, 5, cfg.Server.DrainGraceSec)
	assert.Equal(t, 60, cfg.Server.SyncIntervalMin)
	assert.Equal(t, "postgres", cfg.Database.Engine)
	assert.Equal(t, 3600, cfg.Session.MaxAge)
	assert.True(t, cfg.Session.Secure)
	assert.Equal(t, "grpc", cfg.Otlp.Protocol)
	assert.False(t, cfg.Metrics.Enabled)
}

func TestEnvOverridesFile(t *testing.T) {
	t.Setenv(EnvPrefix+"_SERVER_PORT", "9999")
	t.Setenv(EnvPrefix+"_DATABASE_URL", "postgres://env@localhost:5432/env")

	cfg := mustLoad(t, validYAML)

	assert.Equal(t, 9999, cfg.Server.Port)
	assert.Equal(t, "postgres://env@localhost:5432/env", cfg.Database.URL)
}

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		mutate  func(*Config)
		wantErr string
	}{
		"baseline is valid":     {func(*Config) {}, ""},
		"port zero":             {func(c *Config) { c.Server.Port = 0 }, "server.port"},
		"port too high":         {func(c *Config) { c.Server.Port = 70000 }, "server.port"},
		"timeout zero":          {func(c *Config) { c.Server.Timeout = 0 }, "server.timeout"},
		"schemeless base url":   {func(c *Config) { c.Server.BaseURL = "localhost:8080" }, "server.base_url"},
		"negative drain grace":  {func(c *Config) { c.Server.DrainGraceSec = -1 }, "server.drain_grace_sec"},
		"sync interval zero":    {func(c *Config) { c.Server.SyncIntervalMin = 0 }, "server.sync_interval_min"},
		"bad cron spec":         {func(c *Config) { c.Server.SyncAt = "not a cron" }, "server.sync_at"},
		"too dense cron spec":   {func(c *Config) { c.Server.SyncAt = "*/5 * * * *" }, "minimum gap"},
		"valid cron spec":       {func(c *Config) { c.Server.SyncAt = "0 3 * * *" }, ""},
		"non-postgres engine":   {func(c *Config) { c.Database.Engine = "mysql" }, "database.engine"},
		"missing database url":  {func(c *Config) { c.Database.URL = "" }, "database.url"},
		"negative pool size":    {func(c *Config) { c.Database.PoolMaxConns = -1 }, "database.pool_max_conns"},
		"negative session age":  {func(c *Config) { c.Session.MaxAge = -1 }, "session.max_age"},
		"unknown otlp protocol": {func(c *Config) { c.Otlp.Protocol = "quic" }, "otlp.protocol"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validBaseline()
			tc.mutate(cfg)

			err := cfg.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestSanitizeStripsOtlpScheme(t *testing.T) {
	for _, raw := range []string{"http://collector:4317", "https://collector:4317", "collector:4317"} {
		cfg := &Config{}
		cfg.Otlp.Url = raw
		require.NoError(t, cfg.Sanitize())
		assert.Equal(t, "collector:4317", cfg.Otlp.Url)
	}
}
