package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnknownWriteWaits(t *testing.T) {
	require.Equal(t, "wait_user", RecoveryAction(RecoveryFacts{}))
	require.Equal(t, "wait_user", RecoveryAction(RecoveryFacts{Idempotent: true}))
	require.Equal(t, "reuse", RecoveryAction(RecoveryFacts{HasResult: true}))
	require.Equal(t, "query", RecoveryAction(RecoveryFacts{Queryable: true}))
}

func TestToolRecoveryUsesOnlySafeEvidence(t *testing.T) {
	tests := []struct {
		name  string
		facts RecoveryFacts
		want  string
	}{
		{name: "undispatched", facts: RecoveryFacts{NotDispatched: true}, want: "execute"},
		{name: "valid idempotency key", facts: RecoveryFacts{Idempotent: true, KeyValid: true}, want: "retry"},
		{name: "read only declaration", facts: RecoveryFacts{ReadOnly: true}, want: "retry"},
		{name: "query precedes retry", facts: RecoveryFacts{Queryable: true, ReadOnly: true}, want: "query"},
		{
			name:  "result precedes every recovery path",
			facts: RecoveryFacts{HasResult: true, Queryable: true, ReadOnly: true}, want: "reuse",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, RecoveryAction(test.facts))
		})
	}
}
