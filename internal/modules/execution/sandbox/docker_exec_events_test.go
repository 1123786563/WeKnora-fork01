package sandbox

import (
	"testing"
	"time"

	"github.com/moby/moby/api/types/events"
)

func execEvent(action, containerID, execID string, timeNano int64) events.Message {
	return events.Message{
		Type:   events.ContainerEventType,
		Action: events.Action(action),
		Actor: events.Actor{
			ID:         containerID,
			Attributes: map[string]string{"execID": execID},
		},
		Time:     timeNano / int64(time.Second),
		TimeNano: timeNano,
	}
}

const (
	eventTestContainer = "container-a"
	eventTestExec      = "exec-1"
	eventTestBase      = int64(1758796800_000000000)
)

func TestPairDockerExecEventsCapturesExactPair(t *testing.T) {
	pair, status := pairDockerExecEvents([]events.Message{
		execEvent("exec_start: sh -c true", eventTestContainer, eventTestExec, eventTestBase),
		execEvent("exec_die: sh -c true", eventTestContainer, eventTestExec, eventTestBase+1500_000_000),
	}, eventTestContainer, eventTestExec)
	if status != DockerExecEventPairCaptured {
		t.Fatalf("expected captured status, got %q", status)
	}
	if pair.ExecID != eventTestExec || pair.StartedNano != eventTestBase || pair.FinishedNano != eventTestBase+1500_000_000 {
		t.Fatalf("unexpected pair %+v", pair)
	}
	if d, ok := pair.Duration(); !ok || d.Milliseconds() != 1500 {
		t.Fatalf("expected 1500ms duration, got %v ok=%v", d, ok)
	}
}

func TestPairDockerExecEventsIgnoresOtherExecsAndContainers(t *testing.T) {
	pair, status := pairDockerExecEvents([]events.Message{
		execEvent("exec_start: sh -c true", eventTestContainer, "exec-other", eventTestBase),
		execEvent("exec_start: sh -c true", "container-b", eventTestExec, eventTestBase),
		execEvent("exec_die: sh -c true", eventTestContainer, "exec-other", eventTestBase+1),
	}, eventTestContainer, eventTestExec)
	if status != DockerExecEventPairUnavailable {
		t.Fatalf("expected unavailable for foreign events, got %q pair %+v", status, pair)
	}
	if _, ok := pair.Duration(); ok {
		t.Fatal("foreign events must not yield a duration")
	}
}

func TestPairDockerExecEventsMissingStartIsUnavailable(t *testing.T) {
	_, status := pairDockerExecEvents([]events.Message{
		execEvent("exec_die: sh -c true", eventTestContainer, eventTestExec, eventTestBase),
	}, eventTestContainer, eventTestExec)
	if status != DockerExecEventPairUnavailable {
		t.Fatalf("expected unavailable for missing exec_start, got %q", status)
	}
}

func TestPairDockerExecEventsMissingDieIsUnavailable(t *testing.T) {
	_, status := pairDockerExecEvents([]events.Message{
		execEvent("exec_start: sh -c true", eventTestContainer, eventTestExec, eventTestBase),
	}, eventTestContainer, eventTestExec)
	if status != DockerExecEventPairUnavailable {
		t.Fatalf("expected unavailable for missing exec_die, got %q", status)
	}
}

func TestPairDockerExecEventsIdenticalReplayCollapses(t *testing.T) {
	pair, status := pairDockerExecEvents([]events.Message{
		execEvent("exec_start: sh", eventTestContainer, eventTestExec, eventTestBase),
		execEvent("exec_start: sh", eventTestContainer, eventTestExec, eventTestBase),
		execEvent("exec_die: sh", eventTestContainer, eventTestExec, eventTestBase+5),
		execEvent("exec_die: sh", eventTestContainer, eventTestExec, eventTestBase+5),
	}, eventTestContainer, eventTestExec)
	if status != DockerExecEventPairCaptured {
		t.Fatalf("expected captured for byte-identical duplicates, got %q", status)
	}
	if pair.StartedNano != eventTestBase || pair.FinishedNano != eventTestBase+5 {
		t.Fatalf("unexpected pair %+v", pair)
	}
}

func TestPairDockerExecEventsConflictingDuplicatesUnavailable(t *testing.T) {
	cases := [][]events.Message{
		{
			execEvent("exec_start: sh", eventTestContainer, eventTestExec, eventTestBase),
			execEvent("exec_start: sh", eventTestContainer, eventTestExec, eventTestBase+1),
			execEvent("exec_die: sh", eventTestContainer, eventTestExec, eventTestBase+9),
		},
		{
			execEvent("exec_start: sh", eventTestContainer, eventTestExec, eventTestBase),
			execEvent("exec_die: sh", eventTestContainer, eventTestExec, eventTestBase+9),
			execEvent("exec_die: sh", eventTestContainer, eventTestExec, eventTestBase+11),
		},
	}
	for i, batch := range cases {
		if _, status := pairDockerExecEvents(batch, eventTestContainer, eventTestExec); status != DockerExecEventPairUnavailable {
			t.Fatalf("case %d: expected unavailable for conflicting duplicates, got %q", i, status)
		}
	}
}

func TestPairDockerExecEventsNonMonotonicUnavailable(t *testing.T) {
	_, status := pairDockerExecEvents([]events.Message{
		execEvent("exec_start: sh", eventTestContainer, eventTestExec, eventTestBase+10),
		execEvent("exec_die: sh", eventTestContainer, eventTestExec, eventTestBase),
	}, eventTestContainer, eventTestExec)
	if status != DockerExecEventPairUnavailable {
		t.Fatalf("expected unavailable for non-monotonic pair, got %q", status)
	}
}
