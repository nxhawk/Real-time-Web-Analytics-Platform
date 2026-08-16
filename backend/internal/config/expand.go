package config

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

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
