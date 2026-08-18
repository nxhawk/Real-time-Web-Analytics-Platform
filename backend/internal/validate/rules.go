package validate

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nxhawk/pulse-analytics/backend/internal/model"
)

// eventNameCharset is the character rule from PLAN.md 5.2. The length half of that rule
// lives in Limits.MaxEventNameLen instead of in the pattern, so that the bound is stated
// once and cannot drift from the one the error message quotes.
//
// snake_case only: event_name is LowCardinality(String), and a dictionary that also holds
// "Page View", "pageView" and "page-view" answers three questions where there was one.
var eventNameCharset = regexp.MustCompile(`^[a-z0-9_]+$`)

// eventRule is one step of the pipeline.
//
// A rule reads the client's event, writes the part of the validated event it owns, and
// returns model.ReasonNone or the reason it could not. Rules run in the order of eventRules
// and may assume every earlier rule has already run.
type eventRule struct {
	name  string
	apply func(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason
}

// eventRules is the pipeline, in execution order.
//
// This slice is the extension point: a new rule is a function plus one line here. The test
// suite iterates it and fails when a rule has no case of its own, so the list cannot grow
// unchecked.
var eventRules = []eventRule{
	{"event_name", ruleEventName},
	{"event_id", ruleEventID},
	{"timestamp", ruleTimestamp},
	{"identity", ruleIdentity},
	{"page", rulePage},
	{"referrer", ruleReferrer},
	{"utm", ruleUTM},
	{"audience", ruleAudience},
	{"commerce", ruleCommerce},
	{"properties", ruleProperties},
}

// ruleEventName checks the one field with no server-side fallback. Everything else about an
// event can be repaired or enriched; an event with no name is not an event.
func ruleEventName(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	name := strings.TrimSpace(in.Event)
	switch {
	case name == "":
		return model.ReasonMissingEventName
	case !eventNameCharset.MatchString(name):
		return model.ReasonInvalidEventName
	case len(name) > v.limits.MaxEventNameLen:
		// Safe as a byte count: the charset above is ASCII, so it has already established
		// that one byte is one character.
		return model.ReasonInvalidEventName
	}

	out.EventName = name
	return model.ReasonNone
}

// ruleEventID accepts the client's id or supplies one.
//
// A missing id is repaired rather than rejected: it only has to be unique for query-time
// de-duplication, and a server-side v7 is unique and time-ordered exactly as the client's
// would have been. A malformed id is rejected, because it means the SDK is generating
// something it believes is a UUID and is not — a bug worth reporting back rather than
// papering over for as long as that SDK ships.
func ruleEventID(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	raw := strings.TrimSpace(in.EventID)
	if raw == "" {
		id, err := v.newEventID()
		if err != nil {
			return model.ReasonInvalidEventID
		}
		out.EventID = id
		v.observer.FieldRepaired(string(FieldEventID), string(RepairDefaulted))
		return model.ReasonNone
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return model.ReasonInvalidEventID
	}
	out.EventID = id
	return model.ReasonNone
}

// ruleTimestamp parses the client's instant and corrects a wrong clock.
//
// Skew is corrected rather than rejected because a device with a wrong clock is common and
// its traffic is real; the counter is what keeps the correction from being silent. An
// unparseable string is a different thing — a bug in the client, not a wrong clock — and is
// reported back so it can be fixed.
func ruleTimestamp(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	now := v.now().UTC()

	raw := strings.TrimSpace(in.Timestamp)
	if raw == "" {
		out.EventTime = now
		v.observer.FieldRepaired(string(FieldTimestamp), string(RepairDefaulted))
		return model.ReasonNone
	}

	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return model.ReasonInvalidTimestamp
	}

	// Stored absolutely in UTC; PLAN.md converts to a display zone with toTimeZone at query
	// time. Keeping the client's offset here is how a dashboard ends up seven hours out.
	ts = ts.UTC()

	switch {
	case ts.After(now.Add(v.limits.FutureSkew)):
		out.EventTime = now
		v.observer.ClockSkewed(string(SkewFuture))
	case ts.Before(now.Add(-v.limits.PastSkew)):
		out.EventTime = now
		v.observer.ClockSkewed(string(SkewPast))
	default:
		out.EventTime = ts
	}
	return model.ReasonNone
}

// ruleIdentity bounds the two identifiers.
//
// Both are truncated rather than rejected: an over-long user id still identifies a real
// visitor, and dropping the event would cost a session to save the tail of a string.
func ruleIdentity(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	out.UserID = v.bounded(in.UserID, v.limits.MaxUserIDLen, FieldUserID)

	// An empty session id is the normal case, not a fault: PLAN.md 5.3 stitches one during
	// enrichment from the user id, the date and a 30-minute window.
	out.SessionID = v.bounded(in.SessionID, v.limits.MaxSessionIDLen, FieldSessionID)

	return model.ReasonNone
}

// rulePage sanitises the URL the event happened on.
func rulePage(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	out.Page = v.sanitizedURL(in.Page, v.limits.MaxPageLen, FieldPage)
	return model.ReasonNone
}

// ruleReferrer sanitises the URL the visitor arrived from.
//
// The referrer gets the same denylist as the page, which PLAN.md 5.2 does not ask for. It
// should: a visitor who clicks a link out of a password-reset page sends that page — token
// and all — as the referrer of the next one. Stripping one and storing the other would
// leave the credential in the table by a different column.
func ruleReferrer(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	out.Referrer = v.sanitizedURL(in.Referrer, v.limits.MaxReferrerLen, FieldReferrer)
	return model.ReasonNone
}

// ruleUTM bounds the three campaign labels.
func ruleUTM(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	out.UTMSource = v.bounded(in.UTM.Source, v.limits.MaxLabelLen, FieldUTMSource)
	out.UTMMedium = v.bounded(in.UTM.Medium, v.limits.MaxLabelLen, FieldUTMMedium)
	out.UTMCampaign = v.bounded(in.UTM.Campaign, v.limits.MaxLabelLen, FieldUTMCampaign)
	return model.ReasonNone
}

// ruleAudience normalises who and what the visitor is.
//
// Every field here is repaired rather than rejected, and every one of them is optional in
// the payload because enrichment fills it: GeoIP supplies the country and city, User-Agent
// parsing supplies the device, OS and browser (PLAN.md 5.3). Clearing a value this rule
// cannot trust is what gives enrichment its chance at it.
func ruleAudience(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	if country != "" && !isUpperAlpha(country, 2) {
		country = ""
		v.observer.FieldRepaired(string(FieldCountry), string(RepairStripped))
	}
	out.Country = country

	out.City = v.bounded(in.City, v.limits.MaxCityLen, FieldCity)

	device := model.DeviceType(strings.ToLower(strings.TrimSpace(in.Device)))
	switch {
	case device == "":
		out.Device = model.DeviceUnknown
	case device.Valid():
		out.Device = device
	default:
		out.Device = model.DeviceUnknown
		v.observer.FieldRepaired(string(FieldDevice), string(RepairNormalised))
	}

	out.OS = v.bounded(in.OS, v.limits.MaxLabelLen, FieldOS)
	out.Browser = v.bounded(in.Browser, v.limits.MaxLabelLen, FieldBrowser)

	return model.ReasonNone
}

// ruleCommerce checks the money.
//
// Revenue is the one optional field whose fault rejects the event. ClickHouse silently
// truncates a Decimal(18, 4) that does not fit, and a revenue total that is quietly wrong is
// worse than an event that says which payload was wrong: nobody audits a number that looks
// plausible.
func ruleCommerce(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	revenue := strings.TrimSpace(in.Revenue.String())
	switch {
	case revenue == "":
		out.Revenue = "" // the column DEFAULT is 0
	case !fitsDecimal(revenue, v.limits.RevenuePrecision, v.limits.RevenueScale):
		return model.ReasonInvalidRevenue
	default:
		out.Revenue = json.Number(revenue)
	}

	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	switch {
	case currency == "":
		out.Currency = model.DefaultCurrency
	case isUpperAlpha(currency, 3):
		out.Currency = currency
	default:
		// Not an ISO 4217 code. Falling back to the column default keeps the revenue
		// readable rather than pairing it with a currency nothing can interpret.
		out.Currency = model.DefaultCurrency
		v.observer.FieldRepaired(string(FieldCurrency), string(RepairNormalised))
	}

	return model.ReasonNone
}

// ruleProperties checks the free-form object.
//
// The bound is measured on the serialised bytes because that is what the String column
// stores, and it is checked before anything parses the payload so that an oversized object
// costs a length comparison rather than a parse.
func ruleProperties(v *Validator, in *model.Event, out *model.ValidatedEvent) model.RejectReason {
	raw := bytes.TrimSpace(in.Properties)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		out.Properties = model.EmptyProperties
		return model.ReasonNone
	}

	if len(raw) > v.limits.MaxPropertiesBytes {
		return model.ReasonPropertiesTooLarge
	}

	// An object, not an array and not a scalar. Queries reach into properties by key with
	// the JSONExtract family, and every one of those reads null from an array — a whole
	// dimension that silently returns nothing.
	if raw[0] != '{' {
		return model.ReasonInvalidProperties
	}

	// Compact both validates and canonicalises: whitespace a client indented with is not
	// worth storing on every row, and two spellings of the same object should not compare
	// unequal in a GROUP BY.
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return model.ReasonInvalidProperties
	}

	out.Properties = compact.String()
	return model.ReasonNone
}
