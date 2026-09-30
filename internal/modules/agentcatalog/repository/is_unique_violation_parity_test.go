package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestIsUniqueViolationParity pins the module-local isUniqueViolation copy to
// the semantics of the host original at
// internal/application/repository/voice_session.go:289 (40-workbench keeps the
// original; this is the 4th copy of the family — collapsing it into a single
// implementation is owned by IB2 per conventions §7.1). Drift between the
// copies would silently reclassify unique-index contention in the
// release-publish retry path, so every accepted marker is asserted here.
// Pass B (25c) parity guard — remove_at: ib2.
func TestIsUniqueViolationParity(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error is not a violation", err: nil, want: false},
		{name: "gorm duplicated key", err: gorm.ErrDuplicatedKey, want: true},
		{name: "wrapped gorm duplicated key", err: fmt.Errorf("create release: %w", gorm.ErrDuplicatedKey), want: true},
		{name: "sqlite unique constraint message", err: errors.New("UNIQUE constraint failed: agent_releases.listing_id, agent_releases.release_number"), want: true},
		{name: "postgres duplicate key message", err: errors.New(`duplicate key value violates unique constraint "uq_agent_releases_number"`), want: true},
		{name: "postgres 23505 sqlstate", err: errors.New(`ERROR 23505: unique constraint violated`), want: true},
		{name: "unrelated driver error", err: errors.New("database is locked"), want: false},
		{name: "pointer conflict sentinel", err: errors.New("agent marketplace listing pointer changed"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isUniqueViolation(tc.err))
		})
	}
}
