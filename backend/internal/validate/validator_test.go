package validate

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/model"
)

// TestPartialSuccess is the acceptance criterion of Level 1 in one test: a batch of 100 with
// 3 faults accepts 97 and reports 3, each by its index in the request (PHASES.md 5).
func TestPartialSuccess(t *testing.T) {
	t.Parallel()

	const total = 100
	badIndexes := map[int]model.RejectReason{
		7:  model.ReasonInvalidEventName,
		42: model.ReasonMalformedEvent,
		88: model.ReasonPropertiesTooLarge,
	}

	elements := make([]string, 0, total)
	for i := range total {
		switch i {
		case 7:
			elements = append(elements, `{"event":"Page-View"}`)
		case 42:
			elements = append(elements, `{"event":`) // truncated JSON
		case 88:
			elements = append(elements, `{"event":"page_view","properties":`+
				string(propertiesOfSize(t, DefaultLimits().MaxPropertiesBytes+1))+`}`)
		default:
			elements = append(elements, `{"event":"page_view"}`)
		}
	}

	v, rec := newTestValidator(Options{})

	result, err := v.Validate(testSiteID, requestOf(elements...))

	require.NoError(t, err)
	assert.Len(t, result.Accepted, total-len(badIndexes))
	require.Len(t, result.Rejected, len(badIndexes))

	for _, rejected := range result.Rejected {
		want, isBad := badIndexes[rejected.Index]
		assert.True(t, isBad, "index %d should have been accepted", rejected.Index)
		assert.Equal(t, want, rejected.Reason)
	}

	rejectedCounts, _, _ := rec.snapshot()
	assert.Len(t, rejectedCounts, len(badIndexes), "every rejection is counted once")
}

// TestRequestLevelFaults covers the faults that cost the whole request rather than one event.
func TestRequestLevelFaults(t *testing.T) {
	t.Parallel()

	oneEvent := []json.RawMessage{json.RawMessage(`{"event":"page_view"}`)}

	for _, tc := range []struct {
		name    string
		siteID  string
		request model.IngestRequest
		want    error
	}{
		{
			name:    "no site id at all",
			siteID:  "",
			request: model.IngestRequest{Events: oneEvent},
			want:    ErrSiteIDMismatch,
		},
		{
			name:    "the body claims a site the key does not authorise",
			siteID:  testSiteID,
			request: model.IngestRequest{SiteID: "site_other", Events: oneEvent},
			want:    ErrSiteIDMismatch,
		},
		{
			name:    "an omitted body site id defers to the key",
			siteID:  testSiteID,
			request: model.IngestRequest{Events: oneEvent},
			want:    nil,
		},
		{
			name:    "no events",
			siteID:  testSiteID,
			request: model.IngestRequest{SiteID: testSiteID},
			want:    ErrEmptyBatch,
		},
		{
			name:   "one event over the batch limit",
			siteID: testSiteID,
			request: model.IngestRequest{
				SiteID: testSiteID,
				Events: make([]json.RawMessage, DefaultLimits().MaxEventsPerRequest+1),
			},
			want: ErrBatchTooLarge,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v, _ := newTestValidator(Options{})

			_, err := v.Validate(tc.siteID, tc.request)

			if tc.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// TestBatchAtTheLimitIsAccepted pins the other side of the batch bound, so that an off-by-one
// there is a failing test rather than a rejected customer.
func TestBatchAtTheLimitIsAccepted(t *testing.T) {
	t.Parallel()

	limit := DefaultLimits().MaxEventsPerRequest
	elements := make([]string, limit)
	for i := range elements {
		elements[i] = `{"event":"page_view"}`
	}

	v, _ := newTestValidator(Options{})

	result, err := v.Validate(testSiteID, requestOf(elements...))

	require.NoError(t, err)
	assert.Len(t, result.Accepted, limit)
	assert.Empty(t, result.Rejected)
}

// TestSiteIDComesFromTheKey proves the body cannot choose the tenant. Every analytics query
// is filtered by site_id, so a body that could set it would be a cross-tenant write.
func TestSiteIDComesFromTheKey(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{})

	result, err := v.Validate(testSiteID, model.IngestRequest{
		Events: []json.RawMessage{json.RawMessage(`{"event":"page_view","site_id":"site_evil"}`)},
	})

	require.NoError(t, err)
	require.Len(t, result.Accepted, 1)
	assert.Equal(t, testSiteID, result.Accepted[0].SiteID)
}

// TestUnknownFieldsAreAccepted covers the rollout case: a client running an SDK newer than
// this server sends fields it does not know, and its events must still land.
func TestUnknownFieldsAreAccepted(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{})

	result, err := v.Validate(testSiteID, requestOf(
		`{"event":"page_view","viewport":"1920x1080","experiment":{"id":7}}`,
	))

	require.NoError(t, err)
	require.Len(t, result.Accepted, 1)
	assert.Equal(t, "page_view", result.Accepted[0].EventName)
}

// TestMalformedElements covers every shape of element that cannot become an event at all.
func TestMalformedElements(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		element string
	}{
		{"truncated object", `{"event":`},
		{"an array where an object was expected", `["event"]`},
		{"a bare string", `"page_view"`},
		{"a number", `42`},
		{"a field of the wrong type", `{"event":123}`},
		{"nothing at all", ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v, _ := newTestValidator(Options{})

			result, err := v.Validate(testSiteID, requestOf(tc.element))

			require.NoError(t, err)
			assert.Empty(t, result.Accepted)
			require.Len(t, result.Rejected, 1)
			assert.Equal(t, model.ReasonMalformedEvent, result.Rejected[0].Reason)
			assert.Equal(t, 0, result.Rejected[0].Index)
		})
	}
}

// TestNilObserverIsSafe covers the default path: a caller that does not measure must not have
// to supply an Observer, and must not panic for skipping it.
func TestNilObserverIsSafe(t *testing.T) {
	t.Parallel()

	v := New(Options{})

	result, err := v.Validate(testSiteID, requestOf(
		`{"event":"page_view","page":"/x?token=t","timestamp":"1999-01-01T00:00:00Z"}`,
	))

	require.NoError(t, err)
	require.Len(t, result.Accepted, 1)
	assert.Equal(t, "/x", result.Accepted[0].Page)
	assert.NotZero(t, result.Accepted[0].EventID, "the default generator supplies an id")
}

// TestDefaultsAreApplied checks that New fills in what Options left out, since every other
// test pins those values and would not notice them going missing.
func TestDefaultsAreApplied(t *testing.T) {
	t.Parallel()

	v := New(Options{Limits: Limits{MaxPageLen: 10}})

	limits := v.Limits()

	assert.Equal(t, 10, limits.MaxPageLen, "an explicit limit is honoured")
	assert.Equal(t, DefaultLimits().MaxUserIDLen, limits.MaxUserIDLen, "the rest fall back")
	assert.Equal(t, DefaultLimits().FutureSkew, limits.FutureSkew)
}

// TestLimitsAreOverridable proves the bounds are data rather than constants: shrinking one
// changes behaviour without touching a rule.
func TestLimitsAreOverridable(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{Limits: Limits{MaxUserIDLen: 4, MaxPropertiesBytes: 16}})

	accepted, reason := validateOneEvent(t, v, model.Event{
		Event:  "page_view",
		UserID: "u_123456",
	})
	require.Equal(t, model.ReasonNone, reason)
	assert.Equal(t, "u_12", accepted.UserID)

	_, reason = validateOneEvent(t, v, model.Event{
		Event:      "page_view",
		Properties: json.RawMessage(`{"key":"a value that is well over sixteen bytes"}`),
	})
	assert.Equal(t, model.ReasonPropertiesTooLarge, reason)
}

// TestValidatorIsSafeForConcurrentUse backs the promise in the type's documentation. It is
// worth a test only because `make check` runs with the race detector; without -race it
// proves nothing.
func TestValidatorIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	v, rec := newTestValidator(Options{})

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			result, err := v.Validate(testSiteID, requestOf(
				`{"event":"page_view","page":"/p?token=x"}`,
				`{"event":"BAD"}`,
			))
			assert.NoError(t, err)
			assert.Len(t, result.Accepted, 1)
			assert.Len(t, result.Rejected, 1)
		}()
	}
	wg.Wait()

	rejected, _, _ := rec.snapshot()
	assert.Len(t, rejected, goroutines)
}

// TestAcceptedEventMapsOntoTheColumns walks one fully populated event end to end, so that a
// field silently dropped from the pipeline shows up here rather than as an empty column in
// production.
func TestAcceptedEventMapsOntoTheColumns(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{})

	got, reason := validateOneEvent(t, v, model.Event{
		EventID:    "0192f8a1-2222-7333-8444-555555555555",
		Event:      "purchase",
		UserID:     "u_123",
		SessionID:  "s_456",
		Timestamp:  "2026-08-18T11:00:00.500Z",
		Page:       "/checkout/success?order=42",
		Referrer:   "https://google.com/",
		UTM:        model.UTM{Source: "google", Medium: "cpc", Campaign: "summer"},
		Device:     "desktop",
		OS:         "macOS",
		Browser:    "Chrome",
		Screen:     "1920x1080",
		Country:    "vn",
		City:       "Ho Chi Minh City",
		Revenue:    "199000",
		Currency:   "vnd",
		Properties: json.RawMessage(`{"product_id":"123"}`),
	})

	require.Equal(t, model.ReasonNone, reason)

	// The instant is compared by what it denotes rather than by the representation
	// reflect.DeepEqual would look at, then cleared so the rest can be compared in one go.
	want := time.Date(2026, time.August, 18, 11, 0, 0, 500_000_000, time.UTC)
	assert.True(t, got.EventTime.Equal(want), "event_time: want %s, got %s", want, got.EventTime)
	got.EventTime = time.Time{}

	assert.Equal(t, model.ValidatedEvent{
		SiteID:      testSiteID,
		EventID:     uuid.MustParse("0192f8a1-2222-7333-8444-555555555555"),
		EventName:   "purchase",
		UserID:      "u_123",
		SessionID:   "s_456",
		Page:        "/checkout/success?order=42",
		Referrer:    "https://google.com/",
		UTMSource:   "google",
		UTMMedium:   "cpc",
		UTMCampaign: "summer",
		Country:     "VN",
		City:        "Ho Chi Minh City",
		Device:      model.DeviceDesktop,
		OS:          "macOS",
		Browser:     "Chrome",
		Revenue:     json.Number("199000"),
		Currency:    "VND",
		Properties:  `{"product_id":"123"}`,
	}, got)

	assert.True(t, got.IngestedAt.IsZero(),
		"ingested_at belongs to whatever writes the row, not to validation")
}

// TestScreenIsAcceptedButNotStored records a gap between PLAN.md 5.1 and 6.1 rather than
// leaving it to be rediscovered: the payload documents a screen field and the table has no
// column for it.
func TestScreenIsAcceptedButNotStored(t *testing.T) {
	t.Parallel()

	v, _ := newTestValidator(Options{})

	result, err := v.Validate(testSiteID, requestOf(`{"event":"page_view","screen":"1920x1080"}`))

	require.NoError(t, err)
	assert.Len(t, result.Accepted, 1)
	assert.Empty(t, result.Rejected)
}

// TestLongPageWithASensitiveParameterRecordsBothRepairs covers the case where one field needs
// two different corrections, which a single-repair-per-field design would have lost.
func TestLongPageWithASensitiveParameterRecordsBothRepairs(t *testing.T) {
	t.Parallel()

	v, rec := newTestValidator(Options{})

	got, reason := validateOneEvent(t, v, model.Event{
		Event:     "page_view",
		EventID:   "0192f8a1-2222-7333-8444-555555555555",
		Timestamp: "2026-08-18T12:00:00Z",
		Page:      "/" + repeat("p", 3000) + "?token=secret&keep=1",
	})

	require.Equal(t, model.ReasonNone, reason)
	assert.Len(t, got.Page, DefaultLimits().MaxPageLen)

	_, repaired, _ := rec.snapshot()
	assert.Equal(t, []string{"page:stripped", "page:truncated"}, repaired)
}
