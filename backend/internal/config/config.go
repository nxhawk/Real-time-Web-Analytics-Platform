package config

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

// section is one block of the configuration file. Every field of Config implements it, which
// is what keeps a rule next to the struct it constrains instead of in one function that
// grows with every feature.
//
// A section validates only what it can see. Anything that compares two sections is a
// cross-section rule and belongs in Validate below.
type section interface {
	validate(p *problems)
}

// sections lists the blocks in the order their problems are reported. This is the one place
// a new section has to be registered.
func (c *Config) sections() []section {
	return []section{c.App, c.Log, c.HTTP, c.ClickHouse, c.Ingest, c.Kafka}
}

// Validate reports every problem it can find, not just the first one, so a misconfigured
// deployment can be fixed in one pass. Messages name the YAML key and, where one exists, the
// environment variable that feeds it.
func (c *Config) Validate() error {
	var p problems

	for _, s := range c.sections() {
		s.validate(&p)
	}

	// Cross-section rules. Producing to Kafka without a broker list would start cleanly and
	// then drop every event, so it is refused at startup instead.
	if c.Ingest.Sink == SinkKafka && !c.Kafka.Enabled() {
		p.add("ingest.sink=kafka requires kafka.brokers (KAFKA_BROKERS) to be set")
	}

	return p.err()
}

// IsProduction reports whether the process runs with production behaviour (release-mode
// router, stricter CORS, no debug endpoints).
func (c *Config) IsProduction() bool { return c.App.Env == EnvProduction }
