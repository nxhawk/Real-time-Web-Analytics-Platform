package config

// Kafka holds the Level 4 pipeline settings. Empty Brokers means Kafka is not configured.
type Kafka struct {
	Brokers   []string `mapstructure:"brokers"`
	TopicRaw  string   `mapstructure:"topic_raw"`
	TopicDLQ  string   `mapstructure:"topic_dlq"`
	GroupID   string   `mapstructure:"group_id"`
	BatchSize int      `mapstructure:"batch_size"`
}

// Enabled reports whether this build has somewhere to produce to. Levels 1-3 run with the
// direct sink and leave the whole section empty.
func (k Kafka) Enabled() bool { return len(k.Brokers) > 0 }

// validate implements section.
//
// Deliberately empty. Every rule this section has so far depends on ingest.sink, and a
// section cannot see its siblings, so those rules live in Config.Validate. Rules that need
// only the kafka: block — a topic name pattern, a consumer batch bound — belong here, and
// the method exists so that adding one does not mean changing the aggregate.
func (k Kafka) validate(_ *problems) {}
