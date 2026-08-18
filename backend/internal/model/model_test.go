package model_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/model"
)

func TestDeviceTypeValid(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   model.DeviceType
		want bool
	}{
		{model.DeviceDesktop, true},
		{model.DeviceMobile, true},
		{model.DeviceTablet, true},
		{model.DeviceBot, true},
		{model.DeviceUnknown, true},
		{"", false},
		{"Desktop", false},
		{"smart_fridge", false},
	} {
		t.Run(string(tc.in), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, tc.in.Valid())
		})
	}
}

func TestRejectReasonValid(t *testing.T) {
	t.Parallel()

	for _, reason := range model.RejectReasons() {
		assert.True(t, reason.Valid(), "%s should be a known reason", reason)
		assert.NotEmpty(t, reason.String(), "a reason that reaches a client must have a code")
	}

	assert.False(t, model.ReasonNone.Valid(), "the absence of a reason is not one of them")
	assert.False(t, model.RejectReason("nope").Valid())
}

// TestClosedSetsAreCopies checks that the accessors hand out copies. Both sets are contracts
// — one is serialised to clients, the other is a metric label — and a caller that could write
// to the backing array would be editing the contract from a distance.
func TestClosedSetsAreCopies(t *testing.T) {
	t.Parallel()

	devices := model.DeviceTypes()
	devices[0] = "tampered"
	assert.Equal(t, model.DeviceDesktop, model.DeviceTypes()[0])

	reasons := model.RejectReasons()
	reasons[0] = "tampered"
	assert.Equal(t, model.ReasonMalformedEvent, model.RejectReasons()[0])
}

// TestRejectedEventWireFormat pins the JSON the 202 body carries. The field names are the API
// contract in PLAN.md 5.2, so a rename here is a breaking change and should read as one.
func TestRejectedEventWireFormat(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(model.RejectedEvent{Index: 7, Reason: model.ReasonInvalidEventName})

	require.NoError(t, err)
	assert.JSONEq(t, `{"index":7,"reason":"invalid_event_name"}`, string(raw))
}

// TestIngestRequestKeepsElementsRaw is the decode half of partial success: one unusable
// element must not stop the others from being read.
func TestIngestRequestKeepsElementsRaw(t *testing.T) {
	t.Parallel()

	body := `{"site_id":"site_abc","events":[
		{"event":"page_view"},
		{"event":123},
		{"event":"purchase","revenue":199000}
	]}`

	var req model.IngestRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	require.Len(t, req.Events, 3)
	assert.Equal(t, "site_abc", req.SiteID)

	// The element with a number where a string belongs survives decoding of the envelope and
	// only fails when it is decoded on its own.
	var bad model.Event
	assert.Error(t, json.Unmarshal(req.Events[1], &bad))

	var good model.Event
	require.NoError(t, json.Unmarshal(req.Events[2], &good))
	assert.Equal(t, json.Number("199000"), good.Revenue)
}

// TestEventDecodesTheDocumentedPayload runs the exact example from PLAN.md 5.1 through the
// wire type, so a field renamed in the specification fails here rather than arriving as a
// silent zero value.
func TestEventDecodesTheDocumentedPayload(t *testing.T) {
	t.Parallel()

	body := `{
		"event_id": "0192f8a1-2222-7333-8444-555555555555",
		"event": "page_view",
		"user_id": "u_123",
		"session_id": "s_456",
		"timestamp": "2026-08-11T14:20:00.123Z",
		"page": "/products/123",
		"referrer": "https://google.com/",
		"utm": { "source": "google", "medium": "cpc", "campaign": "summer" },
		"device": "desktop",
		"os": "macOS",
		"browser": "Chrome",
		"screen": "1920x1080",
		"country": "VN",
		"city": "Ho Chi Minh City",
		"revenue": 199000,
		"currency": "VND",
		"properties": { "product_id": "123", "category": "shoes" }
	}`

	var got model.Event
	require.NoError(t, json.Unmarshal([]byte(body), &got))

	assert.Equal(t, "0192f8a1-2222-7333-8444-555555555555", got.EventID)
	assert.Equal(t, "page_view", got.Event)
	assert.Equal(t, "u_123", got.UserID)
	assert.Equal(t, "s_456", got.SessionID)
	assert.Equal(t, "2026-08-11T14:20:00.123Z", got.Timestamp)
	assert.Equal(t, "/products/123", got.Page)
	assert.Equal(t, "https://google.com/", got.Referrer)
	assert.Equal(t, model.UTM{Source: "google", Medium: "cpc", Campaign: "summer"}, got.UTM)
	assert.Equal(t, "desktop", got.Device)
	assert.Equal(t, "macOS", got.OS)
	assert.Equal(t, "Chrome", got.Browser)
	assert.Equal(t, "1920x1080", got.Screen)
	assert.Equal(t, "VN", got.Country)
	assert.Equal(t, "Ho Chi Minh City", got.City)
	assert.Equal(t, json.Number("199000"), got.Revenue)
	assert.Equal(t, "VND", got.Currency)
	assert.JSONEq(t, `{"product_id":"123","category":"shoes"}`, string(got.Properties))
}
