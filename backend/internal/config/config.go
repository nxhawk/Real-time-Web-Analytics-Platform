// Package config loads and validates the runtime configuration of every backend binary.
//
// Configuration comes from exactly two places:
//
//	backend/config/<APP_ENV>.config.yml   the committed, reviewable defaults
//	.env plus the process environment      secrets and per-machine values
//
// The YAML file is the source of truth for every knob. A value that has to differ per
// machine or per deployment is written inside it as a ${VAR} or ${VAR:-fallback}
// placeholder and is resolved from the environment at startup. A placeholder with no
// fallback whose variable is unset is a startup error that names the variable, which is how
// production.config.yml makes CLICKHOUSE_DSN mandatory without any extra Go code.
//
// Real environment variables always win over .env: godotenv.Load never overwrites a
// variable that is already set.
//
// This is the only package allowed to read environment variables. Everything else receives
// a *Config (or a narrower sub-struct) through its constructor, which keeps packages
// testable and makes the full set of knobs discoverable in one place.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	// envFileName is the optional local override file, loaded before the YAML is expanded.
	envFileName = ".env"

	// configDirName is the directory holding the per-environment YAML files.
	configDirName = "config"

	// configDirEnv and envFileEnv let a deployment put the files anywhere. The Docker image
	// sets CONFIG_DIR because the binary runs from / with no source tree around it.
	configDirEnv = "CONFIG_DIR"
	envFileEnv   = "ENV_FILE"

	// searchDepth is how many parent directories are scanned for config/ and .env, so that
	// `go run ./cmd/ingest-api` works from backend/ and from the repository root alike.
	searchDepth = 3
)

// Environment names the deployment stage the process runs in. Its value also selects the
// configuration file: APP_ENV=staging loads config/staging.config.yml.
type Environment string

// The supported deployment stages.
const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
	EnvTest        Environment = "test"
)

// InsertMode selects how events reach ClickHouse. "single" exists only so that the naive
// path stays measurable against the batched path in the Level 3 benchmark.
type InsertMode string

// The supported insert modes.
const (
	InsertModeBatch  InsertMode = "batch"
	InsertModeSingle InsertMode = "single"
)

// Sink selects the transport between the ingest API and ClickHouse.
type Sink string

// The supported sinks.
const (
	SinkDirect Sink = "direct" // ingest API writes to ClickHouse itself (Levels 1-3)
	SinkKafka  Sink = "kafka"  // ingest API produces to Kafka, a consumer writes (Level 4+)
)

// Config is the full configuration surface of every backend binary. Individual binaries use
// the subset they need; keeping one struct means the YAML files stay authoritative.
type Config struct {
	App        App        `mapstructure:"app"`
	HTTP       HTTP       `mapstructure:"http"`
	Log        Log        `mapstructure:"log"`
	ClickHouse ClickHouse `mapstructure:"clickhouse"`
	Ingest     Ingest     `mapstructure:"ingest"`
	Kafka      Kafka      `mapstructure:"kafka"`
}

// App holds process-wide settings.
type App struct {
	Env             Environment   `mapstructure:"env"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// HTTP holds the settings of both HTTP servers.
type HTTP struct {
	IngestAddr         string        `mapstructure:"ingest_addr"`
	AnalyticsAddr      string        `mapstructure:"analytics_addr"`
	ReadHeaderTimeout  time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
	MaxBodyBytes       int64         `mapstructure:"max_body_bytes"` // 1 MiB, see PLAN 5.2
	CORSAllowedOrigins []string      `mapstructure:"cors_allowed_origins"`
}

// Log holds logging settings.
type Log struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | text
}

// ClickHouse holds connection settings for the native protocol (port 9000).
type ClickHouse struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	DialTimeout     time.Duration `mapstructure:"dial_timeout"`
	QueryTimeout    time.Duration `mapstructure:"query_timeout"`
}

// Ingest holds the write-path settings. Defaults come from PHASES.md 2.4 and are re-tuned
// with real numbers in Level 3.
type Ingest struct {
	Sink            Sink       `mapstructure:"sink"`
	InsertMode      InsertMode `mapstructure:"insert_mode"`
	BatchSize       int        `mapstructure:"batch_size"`
	FlushIntervalMS int        `mapstructure:"flush_interval_ms"`
	BufferSize      int        `mapstructure:"buffer_size"`
	Workers         int        `mapstructure:"workers"`
	WALDir          string     `mapstructure:"wal_dir"`
	MaxEventsPerReq int        `mapstructure:"max_events_per_request"`
	RateLimitPerMin int        `mapstructure:"rate_limit_per_min"`
}

// FlushInterval is the maximum time a batch waits before being flushed.
func (i Ingest) FlushInterval() time.Duration {
	return time.Duration(i.FlushIntervalMS) * time.Millisecond
}

// Kafka holds the Level 4 pipeline settings. Empty Brokers means Kafka is not configured.
type Kafka struct {
	Brokers   []string `mapstructure:"brokers"`
	TopicRaw  string   `mapstructure:"topic_raw"`
	TopicDLQ  string   `mapstructure:"topic_dlq"`
	GroupID   string   `mapstructure:"group_id"`
	BatchSize int      `mapstructure:"batch_size"`
}

// Load reads .env when present, loads config/<APP_ENV>.config.yml, resolves every ${VAR}
// placeholder in it, and validates the result. It never returns a partially valid Config.
func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}

	appEnv := Environment(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "" {
		appEnv = EnvDevelopment
	}

	path, err := configFilePath(appEnv)
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if err := expandEnvVars(v); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	// The file name is what selected the environment, so a file claiming to be a different
	// one is a copy-paste mistake worth failing on rather than quietly honouring.
	if cfg.App.Env == "" {
		cfg.App.Env = appEnv
	}
	if cfg.App.Env != appEnv {
		return nil, fmt.Errorf(
			"%s sets app.env=%q but was loaded for APP_ENV=%q", path, cfg.App.Env, appEnv)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", path, err)
	}
	return &cfg, nil
}

// loadDotEnv loads the nearest .env file, if there is one. A missing .env is normal in
// containers and CI, so it is not an error. A malformed one is: silently ignoring it would
// start the process with the wrong settings.
func loadDotEnv() error {
	path := os.Getenv(envFileEnv)
	if path == "" {
		found, ok := searchUp(func(dir string) (string, bool) {
			p := filepath.Join(dir, envFileName)
			return p, isFile(p)
		})
		if !ok {
			return nil
		}
		path = found
	}
	if !isFile(path) {
		return fmt.Errorf("%s=%s: no such file", envFileEnv, path)
	}
	if err := godotenv.Load(path); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	return nil
}

// configFilePath locates <env>.config.yml. CONFIG_DIR wins; otherwise the working directory
// and its parents are searched, which covers `make run` (from backend/) and a bare
// `go test ./...` alike.
func configFilePath(appEnv Environment) (string, error) {
	name := string(appEnv) + ".config.yml"

	if dir := os.Getenv(configDirEnv); dir != "" {
		path := filepath.Join(dir, name)
		if !isFile(path) {
			return "", fmt.Errorf("%s=%s but %s does not exist", configDirEnv, dir, path)
		}
		return path, nil
	}

	found, ok := searchUp(func(dir string) (string, bool) {
		if p := filepath.Join(dir, configDirName, name); isFile(p) {
			return p, true
		}
		// Running from the repository root: the Go module lives one level down.
		p := filepath.Join(dir, "backend", configDirName, name)
		return p, isFile(p)
	})
	if !ok {
		return "", fmt.Errorf(
			"no %s/%s found in the working directory or its %d parents; "+
				"set %s to the directory holding the configuration files",
			configDirName, name, searchDepth, configDirEnv)
	}
	return found, nil
}

// searchUp walks from the working directory towards the filesystem root, calling match on
// each directory until it reports a hit.
func searchUp(match func(dir string) (string, bool)) (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for i := 0; i <= searchDepth; i++ {
		if path, ok := match(dir); ok {
			return path, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// placeholder matches ${VAR} and ${VAR:-fallback}. Anything else, including a bare $VAR, is
// left alone: YAML values legitimately contain dollar signs.
var placeholder = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// expandEnvVars resolves every placeholder in the parsed configuration, in place.
//
// Expansion happens after parsing rather than on the raw file text so that a value
// containing YAML metacharacters — a password with a colon in it, say — cannot change the
// shape of the document. Substituted values stay strings; viper's decoder converts them to
// the field's type, including time.Duration and comma-separated lists.
func expandEnvVars(v *viper.Viper) error {
	var missing []string

	expand := func(key, s string) string {
		return placeholder.ReplaceAllStringFunc(s, func(match string) string {
			groups := placeholder.FindStringSubmatch(match)
			name, fallback := groups[1], groups[2]
			if value, ok := os.LookupEnv(name); ok {
				return value
			}
			// A fallback was written as ${VAR:-...}, so an empty one is deliberate.
			if strings.Contains(match, ":-") {
				return fallback
			}
			missing = append(missing, fmt.Sprintf("%s (required by %s)", name, key))
			return ""
		})
	}

	for _, key := range v.AllKeys() {
		switch value := v.Get(key).(type) {
		case string:
			if value != "" {
				v.Set(key, expand(key, value))
			}
		case []any:
			expanded := make([]string, len(value))
			for i, item := range value {
				s, ok := item.(string)
				if !ok {
					expanded[i] = fmt.Sprint(item)
					continue
				}
				expanded[i] = expand(key, s)
			}
			v.Set(key, expanded)
		}
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf(
			"unset environment variables with no fallback: %s", strings.Join(missing, "; "))
	}
	return nil
}

// Validate reports every problem it can find, not just the first one, so a misconfigured
// deployment can be fixed in one pass. Messages name the YAML key and, where one exists, the
// environment variable that feeds it.
func (c *Config) Validate() error {
	var problems []string

	switch c.App.Env {
	case EnvDevelopment, EnvStaging, EnvProduction, EnvTest:
	default:
		problems = append(problems, fmt.Sprintf(
			"app.env (APP_ENV) must be one of development|staging|production|test, got %q", c.App.Env))
	}

	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		problems = append(problems, fmt.Sprintf(
			"log.level (LOG_LEVEL) must be one of debug|info|warn|error, got %q", c.Log.Level))
	}

	switch c.Log.Format {
	case "json", "text":
	default:
		problems = append(problems, fmt.Sprintf(
			"log.format (LOG_FORMAT) must be json|text, got %q", c.Log.Format))
	}

	switch c.Ingest.InsertMode {
	case InsertModeBatch, InsertModeSingle:
	default:
		problems = append(problems, fmt.Sprintf(
			"ingest.insert_mode (INSERT_MODE) must be batch|single, got %q", c.Ingest.InsertMode))
	}

	switch c.Ingest.Sink {
	case SinkDirect:
	case SinkKafka:
		if len(c.Kafka.Brokers) == 0 {
			problems = append(problems, "ingest.sink=kafka requires kafka.brokers (KAFKA_BROKERS) to be set")
		}
	default:
		problems = append(problems, fmt.Sprintf(
			"ingest.sink (SINK) must be direct|kafka, got %q", c.Ingest.Sink))
	}

	if c.HTTP.IngestAddr == "" {
		problems = append(problems, "http.ingest_addr (HTTP_ADDR) must not be empty")
	}
	if c.HTTP.AnalyticsAddr == "" {
		problems = append(problems, "http.analytics_addr (ANALYTICS_ADDR) must not be empty")
	}
	if c.HTTP.MaxBodyBytes <= 0 {
		problems = append(problems, "http.max_body_bytes (HTTP_MAX_BODY_BYTES) must be greater than 0")
	}
	if c.ClickHouse.DSN == "" {
		problems = append(problems, "clickhouse.dsn (CLICKHOUSE_DSN) must not be empty")
	}
	if c.Ingest.BatchSize <= 0 {
		problems = append(problems, "ingest.batch_size (BATCH_SIZE) must be greater than 0")
	}
	if c.Ingest.FlushIntervalMS <= 0 {
		problems = append(problems, "ingest.flush_interval_ms (FLUSH_INTERVAL_MS) must be greater than 0")
	}
	if c.Ingest.BufferSize < c.Ingest.BatchSize {
		problems = append(problems,
			"ingest.buffer_size (BUFFER_SIZE) must be greater than or equal to ingest.batch_size")
	}
	if c.Ingest.Workers <= 0 {
		problems = append(problems, "ingest.workers (INGEST_WORKERS) must be greater than 0")
	}
	if c.Ingest.MaxEventsPerReq <= 0 || c.Ingest.MaxEventsPerReq > 500 {
		problems = append(problems,
			"ingest.max_events_per_request (MAX_EVENTS_PER_REQUEST) must be between 1 and 500")
	}
	if c.App.ShutdownTimeout <= 0 {
		problems = append(problems, "app.shutdown_timeout (SHUTDOWN_TIMEOUT) must be greater than 0")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// IsProduction reports whether the process runs with production behaviour (release-mode
// router, stricter CORS, no debug endpoints).
func (c *Config) IsProduction() bool { return c.App.Env == EnvProduction }

// SlogLevel maps the configured level onto slog's level type.
func (c *Config) SlogLevel() slog.Level {
	switch strings.ToLower(c.Log.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
