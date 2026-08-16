package config

import "time"

// ClickHouse holds connection settings for the native protocol (port 9000).
type ClickHouse struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	DialTimeout     time.Duration `mapstructure:"dial_timeout"`
	QueryTimeout    time.Duration `mapstructure:"query_timeout"`
}

// validate implements section.
//
// Only the DSN is checked. The pool sizes and timeouts fall back to the driver's own
// defaults when they are zero, and the driver is a better judge of those than this package.
func (c ClickHouse) validate(p *problems) {
	if c.DSN == "" {
		p.add("clickhouse.dsn (CLICKHOUSE_DSN) must not be empty")
	}
}
