package validate

import "time"

// Limits holds every bound validation enforces.
//
// They are fields rather than constants for two reasons: a test can shrink one instead of
// building an 8 KB string to prove the check fires, and a future per-plan or per-site limit
// becomes a value rather than a rewrite. Unset fields fall back to DefaultLimits, so a
// caller that cares about one bound does not have to restate the other twelve.
//
// The numbers come from PHASES.md 2.3, the canonical table for anything that appears in
// more than one document. Change them there first, then here.
type Limits struct {
	// MaxEventsPerRequest bounds one batch. Above it the request is refused whole
	// (ErrBatchTooLarge), because a client that ignores the documented batch size is not
	// making a per-event mistake.
	MaxEventsPerRequest int

	// Character bounds. Every one of them is counted in runes, not bytes: the contract is
	// written in characters, and cutting bytes would leave a partial rune that ClickHouse
	// stores as a replacement character.
	MaxEventNameLen int
	MaxUserIDLen    int
	MaxSessionIDLen int
	MaxPageLen      int
	MaxReferrerLen  int

	// MaxLabelLen bounds the values headed for a LowCardinality(String) column — the utm
	// fields, os and browser. Those columns keep a dictionary per part, so an unbounded
	// value costs far more there than the same bytes in a plain String column.
	MaxLabelLen int

	// MaxCityLen is separate from MaxLabelLen because a city name is legitimately longer
	// than a browser name and its cardinality is bounded by geography rather than by what
	// a client chooses to send.
	MaxCityLen int

	// MaxPropertiesBytes bounds the serialised properties object, measured on the bytes
	// that will be stored.
	MaxPropertiesBytes int

	// RevenuePrecision and RevenueScale are the declaration of the revenue column,
	// Decimal(18, 4) in PLAN.md 6.1. They live here so that a migration changing the column
	// has one obvious place to change the check that guards it.
	RevenuePrecision int
	RevenueScale     int

	// FutureSkew and PastSkew are how far a client clock may be wrong before its timestamp
	// is replaced by the server's (PLAN.md 5.2).
	FutureSkew time.Duration
	PastSkew   time.Duration
}

// DefaultLimits is the shipped contract. Everything in it is quoted from PLAN.md 5.2 and
// PHASES.md 2.3 except the label bounds, which exist to protect the LowCardinality
// dictionaries and are documented on the fields above.
func DefaultLimits() Limits {
	return Limits{
		MaxEventsPerRequest: 500,
		MaxEventNameLen:     64,
		MaxUserIDLen:        128,
		MaxSessionIDLen:     128,
		MaxPageLen:          2048,
		MaxReferrerLen:      2048,
		MaxLabelLen:         64,
		MaxCityLen:          128,
		MaxPropertiesBytes:  8 << 10, // 8 KiB
		RevenuePrecision:    18,
		RevenueScale:        4,
		FutureSkew:          24 * time.Hour,
		PastSkew:            30 * 24 * time.Hour,
	}
}

// withDefaults replaces every unset bound with its default.
//
// Zero is treated as "not set" rather than as "reject everything" on purpose: the zero
// Limits is what a caller passes when it has no opinion, and a validator that refused every
// event would be a confusing way to say so. The one value this costs is RevenueScale = 0,
// which a Decimal(18, 0) column would want; that column does not exist, and if a migration
// ever creates one this default has to move with it.
func (l Limits) withDefaults() Limits {
	d := DefaultLimits()

	l.MaxEventsPerRequest = orDefault(l.MaxEventsPerRequest, d.MaxEventsPerRequest)
	l.MaxEventNameLen = orDefault(l.MaxEventNameLen, d.MaxEventNameLen)
	l.MaxUserIDLen = orDefault(l.MaxUserIDLen, d.MaxUserIDLen)
	l.MaxSessionIDLen = orDefault(l.MaxSessionIDLen, d.MaxSessionIDLen)
	l.MaxPageLen = orDefault(l.MaxPageLen, d.MaxPageLen)
	l.MaxReferrerLen = orDefault(l.MaxReferrerLen, d.MaxReferrerLen)
	l.MaxLabelLen = orDefault(l.MaxLabelLen, d.MaxLabelLen)
	l.MaxCityLen = orDefault(l.MaxCityLen, d.MaxCityLen)
	l.MaxPropertiesBytes = orDefault(l.MaxPropertiesBytes, d.MaxPropertiesBytes)
	l.RevenuePrecision = orDefault(l.RevenuePrecision, d.RevenuePrecision)
	l.RevenueScale = orDefault(l.RevenueScale, d.RevenueScale)
	l.FutureSkew = orDefault(l.FutureSkew, d.FutureSkew)
	l.PastSkew = orDefault(l.PastSkew, d.PastSkew)

	return l
}

// orDefault returns value unless it is unset or negative, in which case it returns fallback.
func orDefault[T ~int | ~int64](value, fallback T) T {
	if value <= 0 {
		return fallback
	}
	return value
}
