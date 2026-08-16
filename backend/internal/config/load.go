package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// isFile reports whether path exists and is a regular file.
//
// The Clean call is not decoration. CONFIG_DIR and ENV_FILE reach this function from the
// process environment, and gosec's G703 taint analysis — correctly — refuses to let an
// environment-derived string reach os.Stat unnormalised. Clean is its recognised sanitizer,
// and normalising once here covers every filesystem lookup the package makes.
func isFile(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	return err == nil && !info.IsDir()
}
