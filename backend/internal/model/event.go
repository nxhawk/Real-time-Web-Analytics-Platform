package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EmptyProperties is what an event with no custom properties stores. The column is a
// String and queries reach into it with the JSONExtract family, which reads an empty
// string as a parse error rather than as an absent key.
const EmptyProperties = "{}"

// DefaultCurrency matches the DEFAULT of the currency column in PLAN.md 6.1. It is applied
// here as well so that an event carrying revenue always carries the currency it is in,
// whichever path wrote the row.
const DefaultCurrency = "VND"

// IngestRequest is the body of POST /api/v1/events, for a single event and a batch alike
// (PLAN.md 5.1).
//
// Events stays raw on purpose. Decoding the whole batch into []Event would let one
// malformed element fail the entire request, which is exactly what the partial-success
// contract forbids: 100 events with 3 bad ones must accept 97 (PLAN.md 5.2). Each element
// is decoded on its own instead, so a decode failure costs one index.
type IngestRequest struct {
	SiteID string            `json:"site_id"`
	Events []json.RawMessage `json:"events"`
}

// UTM is the campaign attribution block of an Event.
type UTM struct {
	Source   string `json:"source,omitempty"`
	Medium   string `json:"medium,omitempty"`
	Campaign string `json:"campaign,omitempty"`
}

// Event is one event exactly as a client sends it (PLAN.md 5.1).
//
// Every field is deliberately permissive — a string where the column is an enumeration,
// json.Number where it is a decimal, json.RawMessage where it is an object. A stricter
// type would move the failure into encoding/json, which reports "cannot unmarshal" for the
// whole request and cannot say which of the 100 events was at fault. Narrowing happens in
// internal/validate, which turns an Event into a ValidatedEvent or a RejectedEvent.
type Event struct {
	EventID   string      `json:"event_id,omitempty"`
	Event     string      `json:"event"`
	UserID    string      `json:"user_id,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	Page      string      `json:"page,omitempty"`
	Referrer  string      `json:"referrer,omitempty"`
	UTM       UTM         `json:"utm,omitempty"`
	Device    string      `json:"device,omitempty"`
	OS        string      `json:"os,omitempty"`
	Browser   string      `json:"browser,omitempty"`
	Country   string      `json:"country,omitempty"`
	City      string      `json:"city,omitempty"`
	Revenue   json.Number `json:"revenue,omitempty"`
	Currency  string      `json:"currency,omitempty"`

	// Screen is accepted because PLAN.md 5.1 documents it, but analytics.events has no
	// column for it (PLAN.md 6.1), so nothing carries it forward. A client that needs it
	// should send it inside Properties until a column exists.
	Screen string `json:"screen,omitempty"`

	// Properties stays raw so that its 8 KB bound is measured on the bytes ClickHouse will
	// actually store rather than on a re-serialisation of a decoded map.
	Properties json.RawMessage `json:"properties,omitempty"`
}

// ValidatedEvent is an Event that passed every rule in internal/validate: bounded lengths,
// a sanitised page, a real UTC instant, and a revenue that fits its column.
//
// Its fields mirror the columns of analytics.events (PLAN.md 6.1) one for one and in the
// same order, so the repository appends them positionally with no mapping step in between.
// Only internal/validate constructs a populated one, which is what keeps an unchecked event
// out of storage — the type is the guarantee rather than a convention someone has to
// remember.
type ValidatedEvent struct {
	// Time. EventTime is when the event happened; IngestedAt is when this system saw it,
	// and their difference is the end-to-end lag PLAN.md 14.1 tracks. IngestedAt is left
	// zero here: the moment of ingestion belongs to whatever writes the row, not to the
	// moment it was checked.
	EventTime  time.Time
	IngestedAt time.Time

	// Identity. SiteID comes from the API key, never from the request body.
	SiteID    string
	EventID   uuid.UUID
	EventName string
	UserID    string
	SessionID string // empty until enrichment stitches one (PLAN.md 5.3)

	// Page context.
	Page        string
	Referrer    string
	UTMSource   string
	UTMMedium   string
	UTMCampaign string

	// Audience. Country and City stay empty when the client omits them, so that GeoIP
	// enrichment can fill them from the request IP (PLAN.md 5.3).
	Country string // ISO 3166-1 alpha-2, upper case
	City    string
	Device  DeviceType
	OS      string
	Browser string

	// Commerce. Revenue is the client's decimal literal, kept verbatim so that no binary
	// float ever touches a money value; the repository converts it to the driver's decimal
	// type. An empty Revenue means zero, matching the column DEFAULT.
	Revenue  json.Number
	Currency string // ISO 4217, upper case

	// Free-form. Compact JSON object, never empty: see EmptyProperties.
	Properties string
}

// RejectedEvent names one event validation refused, by its position in the request.
//
// The field names are part of the API contract: they are serialised verbatim into the 202
// body as `rejected: [{index, reason}]` (PLAN.md 5.2).
type RejectedEvent struct {
	Index  int          `json:"index"`
	Reason RejectReason `json:"reason"`
}
