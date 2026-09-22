package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

func nativeBusinessEvent(id, text string) nativecontract.BusinessEvent {
	return nativecontract.BusinessEvent{
		Protocol: nativecontract.EventProtocol, SchemaVersion: nativecontract.ContractVersion,
		EventID: id, TenantID: "1", SessionID: "session/czE", RunID: "run-1",
		Kind: nativecontract.EventTextDelta, Payload: nativecontract.EventPayload{Text: text},
	}
}

func TestNativeEventSequencesAreMonotonicAndIdentityIsIdempotent(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()
	first := nativeCommitIntent(fence, "intent-1", "hash-1")
	first.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-1", "one")}
	receipt, err := coordinator.Commit(ctx, first)
	require.NoError(t, err)
	require.EqualValues(t, 1, receipt.LastSequence)

	second := nativeCommitIntent(fence, "intent-2", "hash-2")
	second.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-2", "two")}
	receipt, err = coordinator.Commit(ctx, second)
	require.NoError(t, err)
	require.EqualValues(t, 2, receipt.LastSequence)

	page, err := coordinator.Read(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, "", 10)
	require.NoError(t, err)
	require.Len(t, page.Events, 2)
	require.Equal(t, "1", page.Events[0].Sequence)
	require.Equal(t, "2", page.Events[1].Sequence)
}

func TestNativeEventIdentityChangedPayloadConflictsAndExpiredCursorIsTyped(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()
	first := nativeCommitIntent(fence, "intent-1", "hash-1")
	first.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-1", "one")}
	_, err := coordinator.Commit(ctx, first)
	require.NoError(t, err)

	changed := nativeCommitIntent(fence, "intent-2", "hash-2")
	changed.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-1", "changed")}
	_, err = coordinator.Commit(ctx, changed)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))

	_, err = coordinator.Read(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, "0", 10)
	require.Equal(t, nativecontract.ErrCursor, failureCode(t, err))
}
