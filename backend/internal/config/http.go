package config

import "time"

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

// validate implements section.
//
// The timeouts are deliberately unchecked: a zero net/http timeout means "no timeout", which
// is a legitimate choice for a local debugging session. The listen addresses are not, because
// an empty one binds every interface on a random port and looks healthy while being
// unreachable.
func (h HTTP) validate(p *problems) {
	if h.IngestAddr == "" {
		p.add("http.ingest_addr (HTTP_ADDR) must not be empty")
	}
	if h.AnalyticsAddr == "" {
		p.add("http.analytics_addr (ANALYTICS_ADDR) must not be empty")
	}
	if h.MaxBodyBytes <= 0 {
		p.add("http.max_body_bytes (HTTP_MAX_BODY_BYTES) must be greater than 0")
	}
}
