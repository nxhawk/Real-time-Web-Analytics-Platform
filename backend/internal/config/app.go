package config

import "time"

// App holds process-wide settings.
type App struct {
	Env             Environment   `mapstructure:"env"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// validate implements section.
func (a App) validate(p *problems) {
	if !a.Env.Valid() {
		p.addf("app.env (APP_ENV) must be one of %s, got %q", oneOf(environments), a.Env)
	}
	if a.ShutdownTimeout <= 0 {
		p.add("app.shutdown_timeout (SHUTDOWN_TIMEOUT) must be greater than 0")
	}
}
