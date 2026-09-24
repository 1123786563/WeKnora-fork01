# T19 Docker exec duration research

## Scope

This note evaluates whether a restricted, outputless Docker `exec` can report an authoritative duration after response loss or daemon restart. It is read-only research; no application code was changed.

## Verified primary evidence

The repository pins Moby API `v1.55.0` and Moby client `v0.5.1` in `go.mod`. The pinned SDK's `ExecInspectResult` contains only `ID`, `ContainerID`, `Running`, `ExitCode`, and `PID`; the underlying `GET /exec/{id}/json` response likewise has no start or finish timestamp. Therefore an `ExecInspect` call cannot reconstruct duration after the fact.

Docker Engine's official API documents the sequence as `POST /containers/{id}/exec` (returns an exec ID), followed by `POST /exec/{id}/start`; start is a hijacked/streaming operation unless detached. `GET /exec/{id}/json` reports state and exit code, but no timing fields. Sources: [Docker Engine API exec reference](https://docs.docker.com/reference/api/engine/version/v1.40/#exec), [Moby pinned ExecInspectResponse](https://github.com/moby/moby/blob/v28.5.2/api/types/container/exec.go).

Docker event messages contain daemon `time` and `timeNano`, container Actor ID, and attributes. Moby emits `exec_start` with an `execID` attribute when the exec is marked running, and emits `exec_die` with `execID` and `exitCode` when the process terminates. This gives an exact exec-ID-correlated pair while the events are available. Sources: [Moby exec start emission](https://github.com/moby/moby/blob/v28.5.2/daemon/exec.go), [Moby exec die emission](https://github.com/moby/moby/blob/v28.5.2/daemon/monitor.go), [Moby event Message type](https://github.com/moby/moby/blob/v28.5.2/api/types/events/events.go).

The official Docker events documentation states that only the last 256 log events are returned. Events are a real-time daemon stream, local to the node; they are not an application-owned durable execution journal. The API's `since`/`until` query can replay events still retained by the daemon, but cannot recover events evicted from the ring or events lost during a daemon/restart/transport gap. Sources: [Docker system events](https://docs.docker.com/reference/cli/docker/system/events/), [Engine API events reference](https://docs.docker.com/reference/api/engine/version/v1.40/#monitor-events).

## Design ruling

`exec_start` + `exec_die` is authoritative enough for a measured duration only when both events are captured and joined by the exact `execID`, and the timestamps are monotonic in the same daemon event source (`duration = die.timeNano - start.timeNano`). `exec_die` alone is insufficient: it proves termination and exit code but supplies no start time. `exec_start` alone is insufficient: it proves launch but not completion. `ExecInspect` can supplement `Running`/`ExitCode`, but cannot supply missing timestamps.

After response loss, a reconnecting worker may query the event stream with `since` and filter `Actor.Attributes.execID`, then inspect the exec. This is a best-effort recovery window, not durable truth. If either event is absent, the daemon restarted, the event ring overflowed, the exec was removed, timestamps are missing/non-monotonic, or more than one event pair matches the ID, the result must be `duration_unavailable` (or equivalent indeterminate status), never a duration inferred from client wall-clock time or `ExecInspect` polling.

The result contract should therefore expose an explicit timing confidence/source, for example:

```text
duration_ms: integer | null
duration_source: "docker_exec_events" | "unavailable"
duration_complete: boolean
```

Only `duration_source=docker_exec_events` and `duration_complete=true` permit a numeric duration. `started_at` and `finished_at` should be daemon event timestamps, not request receipt or response delivery times. A response-loss retry must not create a second exec merely to obtain timing.

## Required implementation/validation evidence

The T19 implementation should subscribe to Docker events before `exec_start`, filter by container and exact `execID` attribute, persist the paired event timestamps in the T19 operation journal, and use `ExecInspect` only for state/exit code. Tests should cover normal pair, response loss with replay inside the 256-event window, missing `exec_start`, missing `exec_die`, duplicate/conflicting events, event-ring eviction, daemon restart gap, and non-monotonic timestamps. The integration run must demonstrate a restricted command with output discarded, exact exec ID correlation, and explicit indeterminate output for an intentionally unavailable event pair.

## Recommendation

Do not promise an always-available exact duration from Docker alone. Adopt the explicit nullable/indeterminate result contract above. Treat event-pair duration as authoritative only within the daemon's retained event horizon; make missing or ambiguous timing visible and fail closed.

