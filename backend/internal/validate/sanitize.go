package validate

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

// builtinSensitiveParams is stripped from every URL no matter how the deployment is
// configured. Configuration adds to this set; nothing removes from it.
//
// The floor is not paranoia. An analytics URL is one of the most common accidental exports
// of a live credential: a password-reset link, a magic-login link and an OAuth callback all
// carry one in the query string, and all three are pages a visitor loads. CLAUDE.md section
// 3 makes stripping them structural rather than configurable, so turning it off is not one
// typo away.
//
// Every entry is unambiguous. Names that are sometimes a credential and sometimes ordinary
// analytics data — code, ref, state — are deliberately absent: a deployment that needs one
// adds it through ingest.sensitive_query_params, where the choice is visible.
var builtinSensitiveParams = []string{
	"access_token",
	"api_key",
	"apikey",
	"authorization",
	"email",
	"id_token",
	"otp",
	"passwd",
	"password",
	"pwd",
	"refresh_token",
	"secret",
	"token",
}

// denylist is the set of query parameters that must never reach storage. It is a type rather
// than a bare map so that the rules ask it a question instead of reaching into it, which is
// what keeps the case-folding in one place.
type denylist map[string]struct{}

// newDenylist merges the configured names into the built-in floor. Matching is
// case-insensitive, so the set is stored lower-cased and looked up the same way.
func newDenylist(extra []string) denylist {
	set := make(denylist, len(builtinSensitiveParams)+len(extra))
	for _, name := range builtinSensitiveParams {
		set[name] = struct{}{}
	}
	for _, name := range extra {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			set[name] = struct{}{}
		}
	}
	return set
}

// has reports whether a query parameter must not be stored.
func (d denylist) has(name string) bool {
	_, found := d[strings.ToLower(name)]
	return found
}

// sanitizeURL removes the fragment and every denylisted query parameter from raw, then
// bounds what is left. It reports whether anything was removed and whether the result was
// cut, so the caller can record the right repair against the right field.
//
// A value that does not parse as a URL is bounded and passed through rather than rejected.
// The page field is free-form in practice — a single-page app sends "/products/123", a
// native client sends whatever it likes — and losing an event over the shape of a string
// that is only ever grouped by would cost real traffic for no gain.
func (d denylist) sanitizeURL(raw string, limit int) (out string, stripped, truncated bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, false
	}

	out, stripped = d.stripFrom(raw)
	out, truncated = truncateRunes(out, limit)
	return out, stripped, truncated
}

// stripFrom drops the fragment and the denylisted query parameters, returning the input
// unchanged when there was nothing to drop.
//
// The unchanged case is returned verbatim on purpose. Re-encoding a URL through url.URL
// reorders its query parameters and normalises its escapes, and page is a grouping key: two
// spellings of the same URL would become two rows in every Top Pages table. Canonicalising
// only the URLs that had to change keeps that cost where it is unavoidable.
func (d denylist) stripFrom(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		// Not something that can be taken apart — url.Parse rejects little beyond control
		// characters. Cut from the first '#' so the fragment guarantee still holds, and
		// leave the rest alone rather than guessing at its encoding.
		if cut, _, found := strings.Cut(raw, "#"); found {
			return cut, true
		}
		return raw, false
	}

	changed := parsed.Fragment != "" || parsed.RawFragment != ""
	parsed.Fragment, parsed.RawFragment = "", ""

	if parsed.RawQuery != "" {
		query := parsed.Query()
		removed := false
		for name := range query {
			if d.has(name) {
				query.Del(name)
				removed = true
			}
		}
		if removed {
			parsed.RawQuery = query.Encode()
			changed = true
		}
	}

	if !changed {
		return raw, false
	}
	return parsed.String(), true
}

// truncateRunes cuts s to at most limit runes and reports whether it had to. A limit of zero
// or less means unbounded.
//
// The bounds in PLAN.md 5.2 are written in characters. Cutting bytes instead would split a
// multi-byte rune, and ClickHouse stores the orphaned tail as a replacement character — a
// corruption that survives every later query.
func truncateRunes(s string, limit int) (string, bool) {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s, false
	}

	seen := 0
	for i := range s { // ranging a string yields rune start offsets
		if seen == limit {
			return s[:i], true
		}
		seen++
	}
	return s, false
}

// isUpperAlpha reports whether s is exactly n upper-case ASCII letters — the shape of an
// ISO 3166-1 alpha-2 country code and of an ISO 4217 currency code.
//
// Which codes are actually assigned is not checked. That list changes, GeoIP is the authority
// on the country codes this deployment will see, and a validator that rejected a newly
// assigned code would drop real traffic until someone shipped a new table.
func isUpperAlpha(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := range n {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// isDigits reports whether s is one or more ASCII digits. The empty string is not.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// fitsDecimal reports whether the literal s can be stored in Decimal(precision, scale)
// without loss.
//
// It works on the literal rather than on a parsed float64, because parsing money through a
// binary float is how 199000.10 becomes 199000.09999999999. Exponent notation ("1e5") is
// refused: it is legal JSON but no client sends money that way, and accepting it would mean
// re-deriving the digit count from a float after all.
func fitsDecimal(s string, precision, scale int) bool {
	s = strings.TrimPrefix(s, "-") // the sign costs no digits

	intPart, fracPart, hasFrac := strings.Cut(s, ".")
	if !isDigits(intPart) {
		return false
	}
	if hasFrac && !isDigits(fracPart) {
		return false
	}
	if len(fracPart) > scale {
		return false
	}

	// ClickHouse stores Decimal(P, S) as an integer of P digits, S of which are fractional,
	// so the integer part gets P-S of them however few decimals were actually written.
	return len(strings.TrimLeft(intPart, "0"))+scale <= precision
}
