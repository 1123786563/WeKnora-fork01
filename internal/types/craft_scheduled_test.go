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

// TestValidateCronExpressionNeverFires pins the never-fires probe (SP3 Task 2
// review Important-1): calendar-impossible dates — February 30/31, the 31st
// of a 30-day month — parse as legal five-field cron but hold no occurrence
// in ANY year (February never has a 30th, so the class is rejected outright,
// not occasionally accepted). robfig's five-year search then answers a ZERO
// time instead of an error; stored through, that is a year-0001 next_run_at
// ticket — permanently due, and the claim CAS would rewrite the same zero
// ticket every sweep (RowsAffected=1), a runaway fire loop under the
// dispatcher. Validation rejects the whole class at every write entrance.
// February 29 stays the accepted rare-fire boundary: it exists every leap
// year, so it always fires inside the search window.
func TestValidateCronExpressionNeverFires(t *testing.T) {
	never := []string{
		"0 0 30 2 *",  // February 30
		"0 0 31 2 *",  // February 31
		"0 0 31 4 *",  // April 31 (30-day month)
		"0 0 31 6 *",  // June 31
		"0 0 31 9 *",  // September 31
		"0 0 31 11 *", // November 31
	}
	for _, expr := range never {
		require.Error(t, ValidateCronExpression(expr), "never fires: %q", expr)
	}
	require.NoError(t, ValidateCronExpression("0 0 29 2 *"),
		"February 29 fires every leap year — rare is not never")
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

// TestNextCronFireNeverFires pins the zero-time guard: a schedule with no
// occurrence after the sweep instant is an ERROR, never a year-0001 ticket
// (the claim sweep's quarantine catches it, the preview degrades to empty).
// The leap-day contrast proves rare-but-real still answers a real fire.
func TestNextCronFireNeverFires(t *testing.T) {
	from := time.Date(2026, 9, 21, 9, 2, 30, 0, time.UTC)

	_, err := NextCronFire("0 0 30 2 *", from)
	require.Error(t, err, "a zero-time answer is an error, never a year-0001 ticket")
	_, err = NextCronFire("0 0 31 4 *", from)
	require.Error(t, err)

	next, err := NextCronFire("0 0 29 2 *", from)
	require.NoError(t, err)
	require.Equal(t, time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC), next)
}
