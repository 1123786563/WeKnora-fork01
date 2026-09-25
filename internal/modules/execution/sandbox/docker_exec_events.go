package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
)

// DockerExecEventPair is the daemon-authored timing evidence for exactly one
// Docker exec. The timestamps come from the daemon event stream, never from
// client wall-clock measurement or inspect polling.
type DockerExecEventPair struct {
	ExecID       string
	StartedNano  int64
	FinishedNano int64
}

// Duration reports the paired event delta only when both timestamps exist and
// are strictly monotonic. Every other combination is explicitly unavailable.
func (p DockerExecEventPair) Duration() (time.Duration, bool) {
	if p.ExecID == "" || p.StartedNano <= 0 || p.FinishedNano <= p.StartedNano {
		return 0, false
	}
	return time.Duration(p.FinishedNano - p.StartedNano), true
}

// DockerExecEventPairStatus makes the difference between a captured pair and
// an explicitly indeterminate one observable; it is never a guess.
type DockerExecEventPairStatus string

const (
	DockerExecEventPairCaptured    DockerExecEventPairStatus = "docker_exec_events"
	DockerExecEventPairUnavailable DockerExecEventPairStatus = "unavailable"
)

// DockerExecEventPairObserver replays daemon exec events for one exact receipt
// inside the daemon's retained event horizon. It supplements ExecInspect
// (state/exit code) with the only authoritative timing source Docker exposes.
type DockerExecEventPairObserver interface {
	ObserveExecEventPair(ctx context.Context, receipt DockerOutputlessExecReceipt, since time.Time) (DockerExecEventPair, DockerExecEventPairStatus, error)
}

const (
	dockerExecEventStartActionPrefix = "exec_start"
	dockerExecEventDieActionPrefix   = "exec_die"
	dockerExecEventReplayGrace       = time.Second
)

// pairDockerExecEvents joins exec_start and exec_die by the exact execID and
// container. An exact-duplicate delivery collapses; conflicting timestamps,
// a missing half, or a non-monotonic pair are explicitly unavailable.
func pairDockerExecEvents(batch []events.Message, containerID, execID string) (DockerExecEventPair, DockerExecEventPairStatus) {
	unavailable := DockerExecEventPair{ExecID: execID}
	seenStart := map[int64]bool{}
	seenDie := map[int64]bool{}
	for _, msg := range batch {
		if msg.Actor.ID != containerID || msg.Actor.Attributes["execID"] != execID {
			continue
		}
		action := string(msg.Action)
		if msg.TimeNano <= 0 {
			return unavailable, DockerExecEventPairUnavailable
		}
		switch {
		case strings.HasPrefix(action, dockerExecEventStartActionPrefix):
			seenStart[msg.TimeNano] = true
		case strings.HasPrefix(action, dockerExecEventDieActionPrefix):
			seenDie[msg.TimeNano] = true
		default:
			// Unrelated container events (attach, die of the container, ...) are
			// ignored: only the exact exec pair carries this exec's timing.
		}
	}
	if len(seenStart) != 1 || len(seenDie) != 1 {
		return unavailable, DockerExecEventPairUnavailable
	}
	var startNano, dieNano int64
	for nano := range seenStart {
		startNano = nano
	}
	for nano := range seenDie {
		dieNano = nano
	}
	pair := DockerExecEventPair{ExecID: execID, StartedNano: startNano, FinishedNano: dieNano}
	if _, ok := pair.Duration(); !ok {
		return unavailable, DockerExecEventPairUnavailable
	}
	return pair, DockerExecEventPairCaptured
}

// ObserveExecEventPair replays the daemon event ring for one exact receipt.
// The window starts strictly before the durable send claim so both events of
// the claimed physical start are inside it; eviction, a daemon restart gap or
// an ambiguous pair returns the explicit unavailable status.
func (c *DockerRemoteClient) ObserveExecEventPair(ctx context.Context, receipt DockerOutputlessExecReceipt, since time.Time) (DockerExecEventPair, DockerExecEventPairStatus, error) {
	unavailable := DockerExecEventPair{ExecID: receipt.ExecID}
	if c == nil || c.api == nil || receipt.ContainerID == "" || receipt.ExecID == "" {
		return unavailable, DockerExecEventPairUnavailable, fmt.Errorf("invalid restricted Docker exec receipt")
	}
	api := c.api
	if wrapped, ok := api.(*dockerRPCTimeoutAPI); ok {
		api = wrapped.inner
	}
	streamer, ok := api.(interface {
		Events(ctx context.Context, options client.EventsListOptions) client.EventsResult
	})
	if !ok {
		return unavailable, DockerExecEventPairUnavailable, fmt.Errorf("restricted Docker events replay unsupported")
	}
	if since.IsZero() {
		since = time.Now().Add(-dockerExecEventReplayGrace)
	} else {
		since = since.Add(-dockerExecEventReplayGrace)
	}
	until := time.Now()
	result := streamer.Events(ctx, client.EventsListOptions{
		Since:   formatDockerEventTimestamp(since),
		Until:   formatDockerEventTimestamp(until),
		Filters: client.Filters{"container": {receipt.ContainerID: true}},
	})
	batch := make([]events.Message, 0, 2)
	for {
		select {
		case <-ctx.Done():
			return unavailable, DockerExecEventPairUnavailable, ctx.Err()
		case err := <-result.Err:
			if err == nil {
				continue
			}
			if isDockerEventStreamEOF(err) {
				pair, status := pairDockerExecEvents(batch, receipt.ContainerID, receipt.ExecID)
				return pair, status, nil
			}
			return unavailable, DockerExecEventPairUnavailable, dockerError("RestrictedExecEvents", err)
		case msg := <-result.Messages:
			batch = append(batch, msg)
		}
	}
}

func formatDockerEventTimestamp(t time.Time) string {
	return fmt.Sprintf("%d.%09d", t.Unix(), t.Nanosecond())
}

func isDockerEventStreamEOF(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "EOF")
}
