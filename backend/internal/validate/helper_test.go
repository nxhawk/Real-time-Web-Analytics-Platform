package validate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/model"
)

// The tests are internal (package validate rather than validate_test) for one reason: the
// rule registry is the design, and a guard that every entry in it has a case of its own can
// only read the registry from inside. Everything else is exercised through the exported
// API even so.

// testSiteID is the site every fixture belongs to, standing in for what the API-key
// middleware will put in the context at task L1-19.
const testSiteID = "site_test"

// fixedNow is the server clock the suite runs against. Clock skew is a comparison between
// two instants, so pinning one of them is what turns "24 hours in the future" from a race
// against the wall clock into an input.
var fixedNow = time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)

// fixedEventID is what a validator hands to an event that arrived without one, so that an
// accepted event compares equal across runs.
var fixedEventID = uuid.MustParse("0192f8a1-1111-7222-8333-444444444444")

// recorder is an Observer that remembers what it was told, so a test can assert that a
// repair was not only applied but also counted.
//
// It locks because one test hands the same recorder to concurrent goroutines to prove the
// Validator is safe to share; the race detector is what makes that test worth running.
type recorder struct {
	mu       sync.Mutex
	rejected []string
	repaired []string
	skewed   []string
}

func (r *recorder) EventRejected(siteID, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rejected = append(r.rejected, siteID+":"+reason)
}

func (r *recorder) FieldRepaired(field, repair string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.repaired = append(r.repaired, field+":"+repair)
}

func (r *recorder) ClockSkewed(direction string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skewed = append(r.skewed, direction)
}

// snapshot returns the recorded repairs, sorted, so an assertion does not depend on the
// order rules happen to run in.
func (r *recorder) snapshot() (rejected, repaired, skewed []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Copied into non-nil slices so that "nothing happened" compares equal to []string{}
	// rather than making every such assertion spell out nil.
	rejected = append(make([]string, 0, len(r.rejected)), r.rejected...)
	repaired = append(make([]string, 0, len(r.repaired)), r.repaired...)
	skewed = append(make([]string, 0, len(r.skewed)), r.skewed...)
	sort.Strings(rejected)
	sort.Strings(repaired)
	sort.Strings(skewed)
	return rejected, repaired, skewed
}

// newTestValidator builds a Validator with a pinned clock and a pinned id generator, so
// that every accepted event is reproducible. opts is applied first and only its zero fields
// are filled in, which lets a case override the limits or add a denylist entry.
func newTestValidator(opts Options) (*Validator, *recorder) {
	rec := &recorder{}

	if opts.Now == nil {
		opts.Now = func() time.Time { return fixedNow }
	}
	if opts.NewEventID == nil {
		opts.NewEventID = func() (uuid.UUID, error) { return fixedEventID, nil }
	}
	if opts.Observer == nil {
		opts.Observer = rec
	}

	return New(opts), rec
}

// rawEvent marshals e into the wire form a Validator receives.
func rawEvent(t *testing.T, e model.Event) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(e)
	require.NoError(t, err)
	return raw
}

// requestOf builds a request whose elements are the given JSON documents verbatim, so a
// test can hand the validator bytes that no Go struct could produce.
func requestOf(elements ...string) model.IngestRequest {
	req := model.IngestRequest{
		SiteID: testSiteID,
		Events: make([]json.RawMessage, 0, len(elements)),
	}
	for _, element := range elements {
		req.Events = append(req.Events, json.RawMessage(element))
	}
	return req
}

// validateOneEvent runs the whole pipeline over a single event and returns what came out.
// It is the shape almost every rule case needs: one event in, one verdict out.
func validateOneEvent(t *testing.T, v *Validator, e model.Event) (model.ValidatedEvent, model.RejectReason) {
	t.Helper()

	result, err := v.Validate(testSiteID, model.IngestRequest{
		SiteID: testSiteID,
		Events: []json.RawMessage{rawEvent(t, e)},
	})
	require.NoError(t, err)

	if len(result.Rejected) == 1 {
		return model.ValidatedEvent{}, result.Rejected[0].Reason
	}
	require.Len(t, result.Accepted, 1, "expected exactly one verdict")
	return result.Accepted[0], model.ReasonNone
}

// repeat builds a string of n copies of s, for the length-limit cases.
func repeat(s string, n int) string { return strings.Repeat(s, n) }

// propertiesOfSize builds a JSON object whose serialised form is exactly size bytes, for the
// cases either side of the 8 KB bound.
func propertiesOfSize(t *testing.T, size int) json.RawMessage {
	t.Helper()

	const envelope = `{"k":""}` // the shortest object with one string value
	require.Greater(t, size, len(envelope), "size must leave room for the envelope")

	raw := fmt.Sprintf(`{"k":"%s"}`, repeat("a", size-len(envelope)))
	require.Len(t, raw, size)
	return json.RawMessage(raw)
}
