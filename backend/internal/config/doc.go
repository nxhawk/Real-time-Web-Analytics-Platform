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
//
// # File layout
//
// One file per concern, and one file per configuration section. Adding a section means
// adding a file and one line to Config.sections; nothing else in the package changes.
//
//	doc.go          this overview
//	config.go       the Config aggregate, its section list and cross-section rules
//	problems.go     the validation collector shared by every section
//	enums.go        Environment, InsertMode and Sink, plus the sets of allowed values
//	app.go          the app: section
//	http.go         the http: section
//	log.go          the log: section
//	clickhouse.go   the clickhouse: section
//	ingest.go       the ingest: section
//	kafka.go        the kafka: section
//	load.go         Load and the file lookup: .env, CONFIG_DIR, the upward search
//	expand.go       ${VAR} and ${VAR:-fallback} resolution
//
// # Adding a knob
//
//  1. Add the field, with its mapstructure tag, to the section struct it belongs to.
//  2. Add the key to all four files in backend/config/, as a ${VAR:-fallback} placeholder
//     unless the value is a secret that must be mandatory.
//  3. Extend that section's validate method — not Config.Validate, which only owns rules
//     that span two sections.
//  4. Document it in docs/guide/configuration.md and its Vietnamese mirror.
package config
