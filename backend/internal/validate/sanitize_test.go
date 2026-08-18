package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// These cover the helpers directly. Every one of them is reachable through Validate, but a
// boundary is cheaper to state here than to reconstruct through a whole event, and a failure
// points at the helper instead of at the rule that happened to call it.

func TestTruncateRunes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		in        string
		limit     int
		want      string
		truncated bool
	}{
		{"under the limit", "abc", 5, "abc", false},
		{"exactly at the limit", "abcde", 5, "abcde", false},
		{"one over the limit", "abcdef", 5, "abcde", true},
		{"empty input", "", 5, "", false},
		{"a zero limit means unbounded", "abcdef", 0, "abcdef", false},
		{"a negative limit means unbounded", "abcdef", -1, "abcdef", false},
		{"multi-byte runes are counted, not bytes", "éééé", 2, "éé", true},
		{"a cut never lands inside a rune", "日本語です", 3, "日本語", true},
		{"an emoji is one rune", "a👍b", 2, "a👍", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, truncated := truncateRunes(tc.in, tc.limit)

			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.truncated, truncated)
			assert.True(t, len(got) <= len(tc.in), "truncation never grows the value")
		})
	}
}

func TestFitsDecimal(t *testing.T) {
	t.Parallel()

	// The column is Decimal(18, 4): eighteen digits in total, four of them fractional, so
	// the integer part gets fourteen however few decimals are written.
	const precision, scale = 18, 4

	for _, tc := range []struct {
		name string
		in   string
		want bool
	}{
		{"a plain integer", "199000", true},
		{"zero", "0", true},
		{"leading zeros do not count against the precision", "000199000", true},
		{"four decimal places", "1.0000", true},
		{"fewer decimal places than the scale", "1.5", true},
		{"five decimal places", "1.00001", false},
		{"fourteen integer digits", "12345678901234", true},
		{"fifteen integer digits", "123456789012345", false},
		{"negative", "-199000.5", true},
		{"the sign costs no digits", "-12345678901234", true},
		{"exponent notation", "1e5", false},
		{"a trailing dot", "1.", false},
		{"a leading dot", ".5", false},
		{"not a number", "abc", false},
		{"empty", "", false},
		{"a thousands separator", "199,000", false},
		{"two dots", "1.2.3", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, fitsDecimal(tc.in, precision, scale))
		})
	}
}

func TestIsUpperAlpha(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   string
		n    int
		want bool
	}{
		{"an alpha-2 country code", "VN", 2, true},
		{"an alpha-3 currency code", "VND", 3, true},
		{"lower case", "vn", 2, false},
		{"too short", "V", 2, false},
		{"too long", "VNM", 2, false},
		{"a digit", "V1", 2, false},
		{"empty", "", 2, false},
		{"a two-byte letter is still not two ASCII letters", "Đ", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, isUpperAlpha(tc.in, tc.n))
		})
	}
}

func TestStripFromLeavesCleanURLsAlone(t *testing.T) {
	t.Parallel()

	// A page value is a grouping key. Re-encoding a URL that needed no change would reorder
	// its parameters, and Top Pages would then show one page as two rows.
	set := newDenylist(nil)

	for _, in := range []string{
		"/products/123",
		"/search?q=shoes&sort=price&b=2&a=1",
		"https://example.com/a/b?z=1&y=2",
		"/path with spaces",
		"",
	} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			got, stripped := set.stripFrom(in)

			assert.Equal(t, in, got)
			assert.False(t, stripped)
		})
	}
}

func TestStripFromOnUnparseableInput(t *testing.T) {
	t.Parallel()

	// A control character is about the only thing url.Parse refuses. The value is stored
	// either way, so the fragment guarantee still has to hold for it.
	set := newDenylist(nil)

	t.Run("with a fragment", func(t *testing.T) {
		t.Parallel()

		got, stripped := set.stripFrom("/a\x7f/b#frag")

		assert.Equal(t, "/a\x7f/b", got)
		assert.True(t, stripped)
	})

	t.Run("without a fragment", func(t *testing.T) {
		t.Parallel()

		got, stripped := set.stripFrom("/a\x7f/b")

		assert.Equal(t, "/a\x7f/b", got)
		assert.False(t, stripped)
	})
}

// TestDenylistSanitizeURLBoundsTheResult checks the one thing stripFrom does not: the length
// bound is applied after the strip, not before, so a URL that only fits once its token is
// gone keeps all of what is left.
func TestDenylistSanitizeURLBoundsTheResult(t *testing.T) {
	t.Parallel()

	set := newDenylist(nil)

	got, stripped, truncated := set.sanitizeURL("  /p?token=secret&keep=1  ", 6)

	assert.Equal(t, "/p?kee", got)
	assert.True(t, stripped)
	assert.True(t, truncated)
}

func TestNewDenylist(t *testing.T) {
	t.Parallel()

	set := newDenylist([]string{"  Invite_Code  ", "", "   ", "TOKEN"})

	assert.Contains(t, set, "invite_code", "configured names are trimmed and lower-cased")
	assert.Contains(t, set, "password", "the built-in floor is always present")
	assert.NotContains(t, set, "", "blank entries are dropped, so a trailing comma is harmless")
	assert.Len(t, set, len(builtinSensitiveParams)+1, "a duplicate of a built-in adds nothing")
}
