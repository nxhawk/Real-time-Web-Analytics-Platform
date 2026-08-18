package validate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/model"
)

// ruleCase is one event and what validation must make of it.
//
// rule names the eventRules entry the case exercises. TestEveryRuleHasACase reads that
// field, so a rule added to the pipeline without a case here fails the suite instead of
// shipping unchecked — which is the whole point of the registry being a slice.
type ruleCase struct {
	name  string
	rule  string
	event model.Event

	// reason is what the case expects. ReasonNone means the event is accepted, and field
	// and want then say which value to check.
	reason model.RejectReason

	// field selects the one value the case is about; want is what it must hold. A case that
	// only asserts acceptance leaves both nil.
	field func(model.ValidatedEvent) any
	want  any

	// repairs are the "field:repair" pairs the case must report, sorted. An empty slice
	// asserts that nothing was repaired; nil skips the check.
	repairs []string

	// opts overrides the validator this case runs against, for the limits and denylist
	// cases. Its Now and NewEventID are filled in by newTestValidator.
	opts Options
}

// The pipeline in full. Cases are grouped by the rule they exercise and, inside a group,
// ordered from the ordinary input to the pathological one.
//
// It is one declaration rather than ten because the coverage guard below has to see every
// case in a single slice.
func ruleCases() []ruleCase {
	name := func(e model.ValidatedEvent) any { return e.EventName }
	eventID := func(e model.ValidatedEvent) any { return e.EventID }
	eventTime := func(e model.ValidatedEvent) any { return e.EventTime.Format(time.RFC3339Nano) }
	userID := func(e model.ValidatedEvent) any { return e.UserID }
	sessionID := func(e model.ValidatedEvent) any { return e.SessionID }
	page := func(e model.ValidatedEvent) any { return e.Page }
	referrer := func(e model.ValidatedEvent) any { return e.Referrer }
	utmSource := func(e model.ValidatedEvent) any { return e.UTMSource }
	utmCampaign := func(e model.ValidatedEvent) any { return e.UTMCampaign }
	country := func(e model.ValidatedEvent) any { return e.Country }
	city := func(e model.ValidatedEvent) any { return e.City }
	device := func(e model.ValidatedEvent) any { return e.Device }
	browser := func(e model.ValidatedEvent) any { return e.Browser }
	revenue := func(e model.ValidatedEvent) any { return e.Revenue }
	currency := func(e model.ValidatedEvent) any { return e.Currency }
	properties := func(e model.ValidatedEvent) any { return e.Properties }

	return []ruleCase{
		// --- event_name -----------------------------------------------------------
		{
			name:  "snake_case name is kept",
			rule:  "event_name",
			event: model.Event{Event: "page_view"},
			field: name, want: "page_view",
			repairs: []string{"event_id:defaulted", "timestamp:defaulted"},
		},
		{
			name:  "surrounding whitespace is trimmed",
			rule:  "event_name",
			event: model.Event{Event: "  purchase  "},
			field: name, want: "purchase",
		},
		{
			name:   "a missing name is rejected",
			rule:   "event_name",
			event:  model.Event{},
			reason: model.ReasonMissingEventName,
		},
		{
			name:   "a whitespace-only name is missing, not invalid",
			rule:   "event_name",
			event:  model.Event{Event: "   "},
			reason: model.ReasonMissingEventName,
		},
		{
			name:   "upper case is rejected",
			rule:   "event_name",
			event:  model.Event{Event: "PageView"},
			reason: model.ReasonInvalidEventName,
		},
		{
			name:   "a hyphen is rejected",
			rule:   "event_name",
			event:  model.Event{Event: "page-view"},
			reason: model.ReasonInvalidEventName,
		},
		{
			name:   "a non-ASCII name is rejected",
			rule:   "event_name",
			event:  model.Event{Event: "page_viéw"},
			reason: model.ReasonInvalidEventName,
		},
		{
			name:  "a name of exactly 64 characters is kept",
			rule:  "event_name",
			event: model.Event{Event: repeat("e", 64)},
			field: name, want: repeat("e", 64),
		},
		{
			name:   "a name of 65 characters is rejected, not truncated",
			rule:   "event_name",
			event:  model.Event{Event: repeat("e", 65)},
			reason: model.ReasonInvalidEventName,
		},

		// --- event_id -------------------------------------------------------------
		{
			name:  "a client UUID is kept",
			rule:  "event_id",
			event: model.Event{Event: "page_view", EventID: "0192f8a1-2222-7333-8444-555555555555"},
			field: eventID, want: uuid.MustParse("0192f8a1-2222-7333-8444-555555555555"),
			repairs: []string{"timestamp:defaulted"},
		},
		{
			name:  "a missing id is generated server-side",
			rule:  "event_id",
			event: model.Event{Event: "page_view"},
			field: eventID, want: fixedEventID,
			repairs: []string{"event_id:defaulted", "timestamp:defaulted"},
		},
		{
			name:   "a malformed id is rejected",
			rule:   "event_id",
			event:  model.Event{Event: "page_view", EventID: "not-a-uuid"},
			reason: model.ReasonInvalidEventID,
		},
		{
			name:  "a padded id is trimmed before parsing",
			rule:  "event_id",
			event: model.Event{Event: "page_view", EventID: " 0192f8a1-2222-7333-8444-555555555555 "},
			field: eventID, want: uuid.MustParse("0192f8a1-2222-7333-8444-555555555555"),
		},

		// --- timestamp ------------------------------------------------------------
		{
			name:  "an in-range timestamp is kept to the millisecond",
			rule:  "timestamp",
			event: model.Event{Event: "page_view", Timestamp: "2026-08-18T11:59:59.123Z"},
			field: eventTime, want: "2026-08-18T11:59:59.123Z",
		},
		{
			name:  "an offset is converted to UTC rather than stored as sent",
			rule:  "timestamp",
			event: model.Event{Event: "page_view", Timestamp: "2026-08-18T19:00:00+07:00"},
			field: eventTime, want: "2026-08-18T12:00:00Z",
		},
		{
			name:  "a missing timestamp becomes the server clock",
			rule:  "timestamp",
			event: model.Event{Event: "page_view", EventID: "0192f8a1-2222-7333-8444-555555555555"},
			field: eventTime, want: "2026-08-18T12:00:00Z",
			repairs: []string{"timestamp:defaulted"},
		},
		{
			name:  "exactly 24 hours ahead is still trusted",
			rule:  "timestamp",
			event: model.Event{Event: "page_view", Timestamp: "2026-08-19T12:00:00Z"},
			field: eventTime, want: "2026-08-19T12:00:00Z",
		},
		{
			name:  "more than 24 hours ahead is overridden",
			rule:  "timestamp",
			event: model.Event{Event: "page_view", Timestamp: "2026-08-19T12:00:01Z"},
			field: eventTime, want: "2026-08-18T12:00:00Z",
		},
		{
			name:  "exactly 30 days behind is still trusted",
			rule:  "timestamp",
			event: model.Event{Event: "page_view", Timestamp: "2026-07-19T12:00:00Z"},
			field: eventTime, want: "2026-07-19T12:00:00Z",
		},
		{
			name:  "more than 30 days behind is overridden",
			rule:  "timestamp",
			event: model.Event{Event: "page_view", Timestamp: "2026-07-19T11:59:59Z"},
			field: eventTime, want: "2026-08-18T12:00:00Z",
		},
		{
			name:   "an unparseable timestamp is rejected, not corrected",
			rule:   "timestamp",
			event:  model.Event{Event: "page_view", Timestamp: "18/08/2026 12:00"},
			reason: model.ReasonInvalidTimestamp,
		},

		// --- identity -------------------------------------------------------------
		{
			name:  "a user id within the limit is kept",
			rule:  "identity",
			event: model.Event{Event: "page_view", UserID: "u_123"},
			field: userID, want: "u_123",
		},
		{
			name:  "an over-long user id is truncated, not rejected",
			rule:  "identity",
			event: model.Event{Event: "page_view", UserID: repeat("u", 129)},
			field: userID, want: repeat("u", 128),
			repairs: []string{"event_id:defaulted", "timestamp:defaulted", "user_id:truncated"},
		},
		{
			name:  "truncation counts characters, never bytes",
			rule:  "identity",
			event: model.Event{Event: "page_view", UserID: repeat("é", 200)},
			field: userID, want: repeat("é", 128),
		},
		{
			name:  "a missing session id stays empty for enrichment to fill",
			rule:  "identity",
			event: model.Event{Event: "page_view"},
			field: sessionID, want: "",
		},
		{
			name:  "an over-long session id is truncated",
			rule:  "identity",
			event: model.Event{Event: "page_view", SessionID: repeat("s", 200)},
			field: sessionID, want: repeat("s", 128),
		},

		// --- page -----------------------------------------------------------------
		{
			name:  "an ordinary path is stored verbatim",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/products/123"},
			field: page, want: "/products/123",
		},
		{
			name:  "harmless query parameters keep their order",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/search?q=shoes&sort=price&page=2"},
			field: page, want: "/search?q=shoes&sort=price&page=2",
		},
		{
			name:  "the fragment is dropped",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/docs/guide#installation"},
			field: page, want: "/docs/guide",
		},
		{
			name:  "a reset token is stripped and the rest survives",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/reset?token=abc123&lang=vi"},
			field: page, want: "/reset?lang=vi",
		},
		{
			name:  "the denylist ignores case",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/reset?TOKEN=abc123"},
			field: page, want: "/reset",
		},
		{
			name:  "an email address is stripped",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/invite?email=a@b.com&ref=twitter"},
			field: page, want: "/invite?ref=twitter",
		},
		{
			name:  "a configured parameter is stripped alongside the built-in ones",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/p?invite_code=xyz&utm_source=ads"},
			opts:  Options{SensitiveQueryParams: []string{"Invite_Code"}},
			field: page, want: "/p?utm_source=ads",
		},
		{
			name:  "an over-long page is truncated",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "/" + repeat("p", 3000)},
			field: page, want: "/" + repeat("p", 2047),
		},
		{
			name:  "an absolute URL keeps its host",
			rule:  "page",
			event: model.Event{Event: "page_view", Page: "https://shop.example.com/cart?token=t"},
			field: page, want: "https://shop.example.com/cart",
		},

		// --- referrer -------------------------------------------------------------
		{
			name:  "a referrer is sanitised with the same denylist as the page",
			rule:  "referrer",
			event: model.Event{Event: "page_view", Referrer: "https://mail.example.com/reset?token=leak#top"},
			field: referrer, want: "https://mail.example.com/reset",
		},

		// --- utm ------------------------------------------------------------------
		{
			name:  "campaign labels are kept",
			rule:  "utm",
			event: model.Event{Event: "page_view", UTM: model.UTM{Source: "google", Medium: "cpc", Campaign: "summer"}},
			field: utmSource, want: "google",
		},
		{
			name:  "an over-long campaign is truncated to protect the dictionary",
			rule:  "utm",
			event: model.Event{Event: "page_view", UTM: model.UTM{Campaign: repeat("c", 100)}},
			field: utmCampaign, want: repeat("c", 64),
		},

		// --- audience -------------------------------------------------------------
		{
			name:  "a country code is upper-cased",
			rule:  "audience",
			event: model.Event{Event: "page_view", Country: "vn"},
			field: country, want: "VN",
		},
		{
			name:  "a country that is not alpha-2 is cleared for GeoIP to fill",
			rule:  "audience",
			event: model.Event{Event: "page_view", Country: "Vietnam"},
			field: country, want: "",
			repairs: []string{"country:stripped", "event_id:defaulted", "timestamp:defaulted"},
		},
		{
			name:  "an over-long city is truncated",
			rule:  "audience",
			event: model.Event{Event: "page_view", City: repeat("c", 200)},
			field: city, want: repeat("c", 128),
		},
		{
			name:  "a known device class is kept",
			rule:  "audience",
			event: model.Event{Event: "page_view", Device: "mobile"},
			field: device, want: model.DeviceMobile,
		},
		{
			name:  "a device class is matched case-insensitively",
			rule:  "audience",
			event: model.Event{Event: "page_view", Device: "Desktop"},
			field: device, want: model.DeviceDesktop,
		},
		{
			name:  "a missing device is unknown without counting as a repair",
			rule:  "audience",
			event: model.Event{Event: "page_view", EventID: "0192f8a1-2222-7333-8444-555555555555", Timestamp: "2026-08-18T12:00:00Z"},
			field: device, want: model.DeviceUnknown,
			repairs: []string{},
		},
		{
			name:  "an unrecognised device becomes unknown rather than a new dictionary entry",
			rule:  "audience",
			event: model.Event{Event: "page_view", Device: "smart_fridge"},
			field: device, want: model.DeviceUnknown,
			repairs: []string{"device:normalised", "event_id:defaulted", "timestamp:defaulted"},
		},
		{
			name:  "an over-long browser is truncated",
			rule:  "audience",
			event: model.Event{Event: "page_view", Browser: repeat("b", 100)},
			field: browser, want: repeat("b", 64),
		},

		// --- commerce -------------------------------------------------------------
		{
			name:  "no revenue stays empty so the column default applies",
			rule:  "commerce",
			event: model.Event{Event: "page_view"},
			field: revenue, want: json.Number(""),
		},
		{
			name:  "an integer amount is kept verbatim",
			rule:  "commerce",
			event: model.Event{Event: "purchase", Revenue: "199000"},
			field: revenue, want: json.Number("199000"),
		},
		{
			name:  "four decimal places fit the column",
			rule:  "commerce",
			event: model.Event{Event: "purchase", Revenue: "12.3456"},
			field: revenue, want: json.Number("12.3456"),
		},
		{
			name:   "five decimal places would be truncated by ClickHouse, so the event is rejected",
			rule:   "commerce",
			event:  model.Event{Event: "purchase", Revenue: "12.34567"},
			reason: model.ReasonInvalidRevenue,
		},
		{
			name:  "fourteen integer digits fit Decimal(18, 4)",
			rule:  "commerce",
			event: model.Event{Event: "purchase", Revenue: "12345678901234"},
			field: revenue, want: json.Number("12345678901234"),
		},
		{
			name:   "fifteen integer digits overflow it: the scale claims four of the eighteen",
			rule:   "commerce",
			event:  model.Event{Event: "purchase", Revenue: "123456789012345"},
			reason: model.ReasonInvalidRevenue,
		},
		{
			name:  "a refund is a negative amount, not an error",
			rule:  "commerce",
			event: model.Event{Event: "refund", Revenue: "-199000.5000"},
			field: revenue, want: json.Number("-199000.5000"),
		},
		{
			name:   "exponent notation is rejected rather than re-derived from a float",
			rule:   "commerce",
			event:  model.Event{Event: "purchase", Revenue: "1e5"},
			reason: model.ReasonInvalidRevenue,
		},
		{
			name:  "a missing currency takes the column default",
			rule:  "commerce",
			event: model.Event{Event: "purchase", Revenue: "1000"},
			field: currency, want: model.DefaultCurrency,
		},
		{
			name:  "a currency code is upper-cased",
			rule:  "commerce",
			event: model.Event{Event: "purchase", Revenue: "1000", Currency: "usd"},
			field: currency, want: "USD",
		},
		{
			name:  "a currency that is not alpha-3 falls back to the default",
			rule:  "commerce",
			event: model.Event{Event: "purchase", Revenue: "1000", Currency: "dollars"},
			field: currency, want: model.DefaultCurrency,
			repairs: []string{"currency:normalised", "event_id:defaulted", "timestamp:defaulted"},
		},

		// --- properties -----------------------------------------------------------
		{
			name:  "absent properties become an empty object",
			rule:  "properties",
			event: model.Event{Event: "page_view"},
			field: properties, want: model.EmptyProperties,
		},
		{
			name:  "an explicit null becomes an empty object",
			rule:  "properties",
			event: model.Event{Event: "page_view", Properties: json.RawMessage(`null`)},
			field: properties, want: model.EmptyProperties,
		},
		{
			name:  "an object is kept",
			rule:  "properties",
			event: model.Event{Event: "page_view", Properties: json.RawMessage(`{"product_id":"123"}`)},
			field: properties, want: `{"product_id":"123"}`,
		},
		{
			name:   "an array is rejected: JSONExtract would read null from every key",
			rule:   "properties",
			event:  model.Event{Event: "page_view", Properties: json.RawMessage(`["a","b"]`)},
			reason: model.ReasonInvalidProperties,
		},
		{
			name:   "a scalar is rejected for the same reason",
			rule:   "properties",
			event:  model.Event{Event: "page_view", Properties: json.RawMessage(`"just a string"`)},
			reason: model.ReasonInvalidProperties,
		},
	}
}

func TestEventRules(t *testing.T) {
	t.Parallel()

	// Iterated by index and taken by pointer: ruleCase embeds a whole model.Event plus an
	// Options, so copying one per iteration is half a kilobyte the loop does not need.
	cases := ruleCases()
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.rule+"/"+tc.name, func(t *testing.T) {
			t.Parallel()

			v, rec := newTestValidator(tc.opts)
			got, reason := validateOneEvent(t, v, tc.event)

			require.Equal(t, tc.reason, reason, "reject reason")
			if tc.reason != model.ReasonNone {
				return
			}

			if tc.field != nil {
				assert.Equal(t, tc.want, tc.field(got))
			}
			if tc.repairs != nil {
				_, repaired, _ := rec.snapshot()
				assert.Equal(t, tc.repairs, repaired, "recorded repairs")
			}
		})
	}
}

// TestEveryRuleHasACase is the guard that keeps the table honest. Adding a rule to
// eventRules without a case here fails the suite, which is the only thing that stops the
// registry from growing a step nobody ever exercised.
func TestEveryRuleHasACase(t *testing.T) {
	t.Parallel()

	covered := make(map[string]int, len(eventRules))
	cases := ruleCases()
	for i := range cases {
		covered[cases[i].rule]++
	}

	for _, rule := range eventRules {
		assert.Positive(t, covered[rule.name], "rule %q has no case in ruleCases", rule.name)
	}

	known := make(map[string]bool, len(eventRules))
	for _, rule := range eventRules {
		known[rule.name] = true
	}
	for ruleName := range covered {
		assert.True(t, known[ruleName], "case names rule %q, which is not in eventRules", ruleName)
	}
}

// TestPropertiesAreCompacted needs the request built from literal bytes: json.Marshal
// compacts an embedded json.RawMessage on the way out, so an event built with it could never
// carry the indentation this checks.
func TestPropertiesAreCompacted(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{})

	result, err := v.Validate(testSiteID, requestOf(
		`{"event":"page_view","properties":{ "product_id" : "123" , "category": "shoes" }}`,
	))

	require.NoError(t, err)
	require.Empty(t, result.Rejected)
	require.Len(t, result.Accepted, 1)
	assert.JSONEq(t, `{"product_id":"123","category":"shoes"}`, result.Accepted[0].Properties)
	assert.NotContains(t, result.Accepted[0].Properties, " ")
}

// TestPropertiesSizeBound checks both sides of the 8 KB limit, which needs a payload built
// to the byte rather than a literal in the table.
func TestPropertiesSizeBound(t *testing.T) {
	t.Parallel()

	limit := DefaultLimits().MaxPropertiesBytes

	t.Run("exactly at the limit is accepted", func(t *testing.T) {
		t.Parallel()

		v, _ := newTestValidator(Options{})
		got, reason := validateOneEvent(t, v, model.Event{
			Event:      "page_view",
			Properties: propertiesOfSize(t, limit),
		})

		require.Equal(t, model.ReasonNone, reason)
		assert.Len(t, got.Properties, limit)
	})

	t.Run("one byte over the limit is rejected", func(t *testing.T) {
		t.Parallel()

		v, _ := newTestValidator(Options{})
		_, reason := validateOneEvent(t, v, model.Event{
			Event:      "page_view",
			Properties: propertiesOfSize(t, limit+1),
		})

		assert.Equal(t, model.ReasonPropertiesTooLarge, reason)
	})
}

// TestClockSkewIsCounted proves the correction is visible. PLAN.md 5.2 asks for the counter
// by name precisely because a silent override is indistinguishable from correct data.
func TestClockSkewIsCounted(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		timestamp string
		want      []string
	}{
		{"future", "2026-09-01T00:00:00Z", []string{"future"}},
		{"past", "2026-01-01T00:00:00Z", []string{"past"}},
		{"in range", "2026-08-18T10:00:00Z", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v, rec := newTestValidator(Options{})
			got, reason := validateOneEvent(t, v, model.Event{
				Event: "page_view", Timestamp: tc.timestamp,
			})

			require.Equal(t, model.ReasonNone, reason)
			_, _, skewed := rec.snapshot()
			assert.Equal(t, tc.want, skewed)

			if len(tc.want) > 0 {
				assert.Equal(t, fixedNow.Format(time.RFC3339Nano), got.EventTime.Format(time.RFC3339Nano))
			}
		})
	}
}

// TestEventIDGeneratorFailure covers the branch where the server cannot supply an id. A
// generator that fails is a broken process rather than a bad payload, but returning a reason
// keeps the failure per-event instead of taking the batch down with it.
func TestEventIDGeneratorFailure(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{
		NewEventID: func() (uuid.UUID, error) { return uuid.Nil, assert.AnError },
	})

	_, reason := validateOneEvent(t, v, model.Event{Event: "page_view"})

	assert.Equal(t, model.ReasonInvalidEventID, reason)
}

// TestBuiltinDenylistCannotBeDisabled locks down the one guarantee that is structural rather
// than configurable: configuration adds to the denylist and never removes from it.
func TestBuiltinDenylistCannotBeDisabled(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{SensitiveQueryParams: []string{}})

	got, reason := validateOneEvent(t, v, model.Event{
		Event: "page_view",
		Page:  "/reset?password=hunter2",
	})

	require.Equal(t, model.ReasonNone, reason)
	assert.Equal(t, "/reset", got.Page)
}
