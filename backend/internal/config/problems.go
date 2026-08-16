package config

import (
	"errors"
	"fmt"
	"strings"
)

// problems collects validation failures. Validation reports everything it can find rather
// than stopping at the first fault, so a misconfigured deployment is fixed in one pass
// instead of one restart per typo.
//
// Sections write into a shared collector instead of returning an error each. Returning
// errors would mean joining strings in the section, splitting them again in the aggregate,
// and losing the ability to tell "no problems" from "one empty problem".
type problems struct {
	found []string
}

// add records one problem. The message names the YAML key first and, in brackets, the
// environment variable that feeds it, so the reader can fix either layer:
//
//	ingest.batch_size (BATCH_SIZE) must be greater than 0
func (p *problems) add(message string) {
	p.found = append(p.found, message)
}

// addf is add with formatting. It is a printf wrapper, so `go vet` checks its arguments.
func (p *problems) addf(format string, args ...any) {
	p.found = append(p.found, fmt.Sprintf(format, args...))
}

// err returns nil when nothing was recorded, and otherwise a single error listing every
// problem in the order it was found.
func (p *problems) err() error {
	if len(p.found) == 0 {
		return nil
	}
	return errors.New(strings.Join(p.found, "; "))
}
