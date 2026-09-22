package service

// CFT-S03-T021: the main-agent merge semantics at the delegation service
// seam. The point suites live in craft_delegate_test.go (unknown keeps the
// tool call pending without persisting; observation-only settlement); this
// pins the remaining merge facts:
//   1. a FAILED delegation round never replaces the last delivered version —
//      the delegate service answers failed and persists exactly that; the
//      version store is the container layer's concern and its "failed
//      collection leaves no version" rule is pinned in
//      TestCraftArtifactPublish* + craft_runtime's failed settle
//   2. an UNKNOWN delegation is never folded into a terminal answer — the
//      error surfaces ErrUnknown and nothing is stored
//   3. a succeeded round's artifact.published event is what makes the final
//      answer reference a REAL published version (pinned in the container's
//      emitter payload test); here the delegate result itself stays honest:
//      its status/summary carry only executor facts
import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// A failed sub-round answers failed (the main agent may then repair or
// explain) and never pretends a delivery happened. The expired-deadline
// path is the service's OWN settlement (the executor-terminal path has the
// real executor persist its result before returning — the fake cannot).
func TestCraftDelegateMergeFailedRoundAnswersFailed(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	svc := NewCraftDelegateService(store, exec)

	task := delegateTestTask("failing round")
	task.Deadline = time.Now().Add(-time.Minute) // already past: settle(failed)
	result, err := svc.Delegate(context.Background(), task)
	require.NoError(t, err, "a definitive sub failure is an ANSWER, not an error")
	require.Equal(t, "failed", result.Status)
	require.Contains(t, result.Summary, "deadline")

	// The stored result is exactly the failure — no version-shaped payload,
	// nothing a caller could mistake for a delivery.
	stored, err := store.GetResult(context.Background(), delegateTestScope(), result.TaskID)
	require.NoError(t, err)
	require.Equal(t, "failed", stored.Status)
	require.Empty(t, stored.Files, "a failed round carries no deliverable files")
}

// An unknown delegation never becomes a terminal answer: ErrUnknown
// surfaces and the store stays empty so the main tool call keeps waiting.
func TestCraftDelegateMergeUnknownNeverTerminal(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	exec.onExecute = func(task craft.Task) (craft.Result, error) {
		return craft.Result{TaskID: task.ID, Status: "unknown", Summary: "stream broke"},
			errors.Join(craft.ErrUnknown, errors.New("stream broke"))
	}
	svc := NewCraftDelegateService(store, exec)

	_, err := svc.Delegate(context.Background(), delegateTestTask("unknown round"))
	require.ErrorIs(t, err, craft.ErrUnknown, "unknown must reach the caller as ErrUnknown")

	stored, getErr := store.GetResult(context.Background(), delegateTestScope(), "dlg-u")
	require.Error(t, getErr, "nothing was persisted for the unknown round")
	require.Equal(t, craft.Result{}, stored)
}

// A succeeded round's result carries only executor facts; any
// version/artifact reference in the final answer originates from the
// artifact.published event the container emits on real publication
// (TestCraftRunEventEmitterPayloadShape) — never from the delegate summary.
func TestCraftDelegateMergeSucceededCarriesExecutorFactsOnly(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	exec.onExecute = func(task craft.Task) (craft.Result, error) {
		return craft.Result{TaskID: task.ID, Status: "succeeded", Summary: "index.html written"}, nil
	}
	svc := NewCraftDelegateService(store, exec)

	result, err := svc.Delegate(context.Background(), delegateTestTask("good round"))
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, "index.html written", result.Summary)
	require.Empty(t, result.Files, "the delegate result never fabricates version files")
}
