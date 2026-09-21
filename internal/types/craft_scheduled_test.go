package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestValidateCronExpression pins the canonical five-field contract: the
// standard forms pass, anything else — wrong field count, garbage, or an
// @every descriptor with its sub-minute periods — is rejected.
func TestValidateCronExpression(t *testing.T) {
	valid := []string{
		"*/5 * * * *",
		"0 9 * * *",
		"30 8 1 * *",
		"0 0 * * 1",
		"15 14 1 1 *",
		" 0 9 * * * ", // surrounding whitespace is trimmed
	}
	for _, expr := range valid {
		require.NoError(t, ValidateCronExpression(expr), "expected valid: %s", expr)
	}

	invalid := []string{
		"",
		"   ",
		"not a cron",
		"* * *",            // too few fields
		"* * * * * *",      // six fields (seconds) — not canonical here
		"61 * * * *",       // minute out of range
		"* 25 * * *",       // hour out of range
		"@every 30s",       // descriptor: sub-minute period
		"@daily",           // descriptor: not five fields
		"TZ=UTC 0 9 * * *", // TZ prefix is a six-field parse
	}
	for _, expr := range invalid {
		require.Error(t, ValidateCronExpression(expr), "expected invalid: %q", expr)
	}
}

// TestNextCronFire checks the strictly-after semantics the dispatcher's
// claim relies on: the advance is always past the sweep instant, follows the
// expression's schedule, and invalid input errors instead of guessing.
func TestNextCronFire(t *testing.T) {
	from := time.Date(2026, 9, 21, 9, 2, 30, 0, time.UTC)

	next, err := NextCronFire("*/5 * * * *", from)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 9, 21, 9, 5, 0, 0, time.UTC), next)

	next, err = NextCronFire("0 9 * * *", from)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC), next)

	// Exactly on a boundary: the next fire is strictly after, never the
	// boundary instant itself (a claim must not create a due loop).
	boundary := time.Date(2026, 9, 21, 9, 5, 0, 0, time.UTC)
	next, err = NextCronFire("*/5 * * * *", boundary)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 9, 21, 9, 10, 0, 0, time.UTC), next)

	_, err = NextCronFire("* * *", from)
	require.Error(t, err)
}
