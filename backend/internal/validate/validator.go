package validate

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nxhawk/pulse-analytics/backend/internal/model"
)

// Faults of the request itself, as opposed to a fault of one event inside it.
//
// A model.RejectReason names one bad element of an otherwise usable batch; these mean there
// is nothing to process at all. They are sentinel errors rather than status codes because
// this package does not know it is being called over HTTP — the handler maps them when it
// answers (task L1-17).
var (
	// ErrEmptyBatch means the request carried no events.
	ErrEmptyBatch = errors.New("validate: request contains no events")

	// ErrBatchTooLarge means the request carried more events than Limits.MaxEventsPerRequest.
	ErrBatchTooLarge = errors.New("validate: request exceeds the maximum number of events")

	// ErrSiteIDMismatch means the body claimed a site the API key does not authorise.
	ErrSiteIDMismatch = errors.New("validate: site_id does not match the API key")
)

// Result is the outcome of one ingest request: the events that may be stored, and for every
// event that may not, its position in the request and why.
//
// Both halves are returned together because that is the contract: a batch of 100 with 3
// faults accepts 97 and reports 3 (PLAN.md 5.2).
type Result struct {
	Accepted []model.ValidatedEvent
	Rejected []model.RejectedEvent
}

// Options configures a Validator. Every field has a working default, so
// validate.New(validate.Options{}) is a usable validator.
type Options struct {
	// Limits bounds every field. Unset members fall back to DefaultLimits.
	Limits Limits

	// SensitiveQueryParams are stripped from page and referrer in addition to the built-in
	// floor, which cannot be disabled from here. Matching is case-insensitive.
	SensitiveQueryParams []string

	// Now supplies the server clock. Tests replace it so that clock skew is a fixed input
	// rather than a race against the wall clock.
	Now func() time.Time

	// NewEventID supplies an id for events that arrive without one. Tests replace it to make
	// the accepted events comparable.
	NewEventID func() (uuid.UUID, error)

	// Observer records repairs and rejections. Defaults to doing nothing, so a caller that
	// does not measure does not have to supply one.
	Observer Observer
}

// Validator turns the events of one request into the events that may be stored.
//
// It is immutable after New and safe for concurrent use, so a process builds one at startup
// and shares it across every request goroutine.
type Validator struct {
	limits     Limits
	sensitive  denylist
	now        func() time.Time
	newEventID func() (uuid.UUID, error)
	observer   Observer
}

// New builds a Validator from opts, filling in every default.
func New(opts Options) *Validator {
	v := &Validator{
		limits:     opts.Limits.withDefaults(),
		sensitive:  newDenylist(opts.SensitiveQueryParams),
		now:        opts.Now,
		newEventID: opts.NewEventID,
		observer:   opts.Observer,
	}

	if v.now == nil {
		v.now = time.Now
	}
	if v.newEventID == nil {
		// v7 rather than v4: the id is time-ordered, which keeps inserts into a sorted
		// column store cheap and makes it usable as a cursor tiebreaker.
		v.newEventID = uuid.NewV7
	}
	if v.observer == nil {
		v.observer = nopObserver{}
	}

	return v
}

// Limits reports the bounds this Validator enforces, after defaults were applied. The
// handler needs them to describe its own limits in an error response, and the copy keeps
// that from being a way to change them.
func (v *Validator) Limits() Limits { return v.limits }

// Validate turns one request into the events that may be stored plus the ones that may not.
//
// siteID comes from the API key, never from the body: the key is what the request proved and
// the body may claim any site (CLAUDE.md section 3). A body that names a different site is
// refused rather than silently corrected, because the difference is either an attempt or a
// misconfigured client and both are worth reporting.
//
// A returned error means the request as a whole is unusable. Anything wrong with an
// individual event becomes a model.RejectedEvent instead, so that one bad element never
// costs the other ninety-nine.
func (v *Validator) Validate(siteID string, req model.IngestRequest) (Result, error) {
	switch {
	case siteID == "" || (req.SiteID != "" && req.SiteID != siteID):
		return Result{}, ErrSiteIDMismatch
	case len(req.Events) == 0:
		return Result{}, ErrEmptyBatch
	case len(req.Events) > v.limits.MaxEventsPerRequest:
		return Result{}, ErrBatchTooLarge
	}

	result := Result{Accepted: make([]model.ValidatedEvent, 0, len(req.Events))}

	for i, raw := range req.Events {
		event, reason := v.validateOne(siteID, raw)
		if reason != model.ReasonNone {
			result.Rejected = append(result.Rejected, model.RejectedEvent{Index: i, Reason: reason})
			v.observer.EventRejected(siteID, reason.String())
			continue
		}
		result.Accepted = append(result.Accepted, event)
	}

	return result, nil
}

// validateOne decodes and checks the event at one index of the request.
//
// The element is decoded here rather than alongside the envelope because a decode failure
// has to cost exactly this event: unmarshalling the whole batch at once fails all of them
// and cannot say which one was at fault.
func (v *Validator) validateOne(siteID string, raw json.RawMessage) (model.ValidatedEvent, model.RejectReason) {
	var in model.Event
	if err := json.Unmarshal(raw, &in); err != nil {
		return model.ValidatedEvent{}, model.ReasonMalformedEvent
	}

	// Unknown fields are accepted on purpose. A client running an SDK newer than this server
	// is a normal state during a rollout, and refusing its events would turn a forward-
	// compatible payload into an outage.
	out := model.ValidatedEvent{SiteID: siteID}
	for _, rule := range eventRules {
		if reason := rule.apply(v, &in, &out); reason != model.ReasonNone {
			return model.ValidatedEvent{}, reason
		}
	}

	return out, model.ReasonNone
}

// bounded trims a value, cuts it to limit runes, and records the repair if it had to.
func (v *Validator) bounded(raw string, limit int, field Field) string {
	out, truncated := truncateRunes(strings.TrimSpace(raw), limit)
	if truncated {
		v.observer.FieldRepaired(string(field), string(RepairTruncated))
	}
	return out
}

// sanitizedURL cleans a URL-shaped value and records whichever repairs it needed. A value can
// need both: a page that carried a token and was still too long once it was gone.
func (v *Validator) sanitizedURL(raw string, limit int, field Field) string {
	out, stripped, truncated := v.sensitive.sanitizeURL(raw, limit)
	if stripped {
		v.observer.FieldRepaired(string(field), string(RepairStripped))
	}
	if truncated {
		v.observer.FieldRepaired(string(field), string(RepairTruncated))
	}
	return out
}
