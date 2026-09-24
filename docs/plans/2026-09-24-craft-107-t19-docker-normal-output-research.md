# T19 Docker normal-output research

Date: 2026-09-24  
Scope: read-only research for the integration worktree. No production code, requirements, or remote issues changed.

## Evidence reviewed

- Repository pins `github.com/moby/moby/api v1.55.0` and `github.com/moby/moby/client v0.5.1` (`go.mod:262-263`).
- The pinned client implementation is locally available at
  `/Users/wuyongjun/go/pkg/mod/github.com/moby/moby/client@v0.5.1/container_exec.go`.
- Primary Engine API documentation: [Moby Engine API v1.24, Exec Create/Start/Inspect and stream format](https://github.com/moby/moby/blob/master/api/docs/v1.24.md), and [Moby client `container_exec.go`](https://github.com/moby/moby/blob/master/client/container_exec.go).
- The requested files `docs/plans/2026-09-24-craft-107-t19-docker-s3-provider-design.md`, the S3 review files, and the T19 sandbox Task4b brief are not present in this checkout (bounded search under `/Users/wuyongjun/trea` found no matching files). Conclusions below therefore use the user-provided S3 premise and the pinned SDK/API as facts, while marking integration-specific claims as conditional.

## Verified facts

1. Docker Exec is a two-stage protocol: `POST /containers/{id}/exec` creates an exec instance and returns an ID; `POST /exec/{id}/start` starts that instance. The API documentation explicitly describes `detach=true` as returning after starting the command, and `detach=false` as opening an interactive session.
2. In the pinned Go SDK, `ExecCreateOptions` carries `AttachStdin`, `AttachStdout`, `AttachStderr`, `TTY`, command, environment, and working directory. `ExecStartOptions` only carries `Detach`, `TTY`, and console size. `ExecStart` performs the `/exec/{id}/start` POST and returns an empty result; it has no stream reader or stdin writer.
3. `ExecAttach` is not a passive read operation in this SDK. Its implementation sends another POST to the same `/exec/{id}/start` endpoint with `Detach:false`, then returns a hijacked connection. The SDK comment says the returned connection carries output and must be closed by the caller.
4. For non-TTY execs, Docker multiplexes stdout and stderr in one stream; the official API says to demultiplex using the 8-byte stream header or `stdcopy.StdCopy`. For TTY execs, output is a single raw stream and stdout/stderr separation is unavailable.
5. `ExecInspect` exposes `Running`, `ExitCode`, PID, and IDs. It does not expose buffered stdout, stderr, stdin, or a replay cursor. Once an output-bearing connection is lost, inspect can establish state/exit evidence but cannot reconstruct bytes already emitted.
6. The API's exec start endpoint accepts only `Detach` and `Tty` in its request body. There is no API option on `ExecStart` to choose stdout/stderr/stdin after `ExecCreate`; stream attachment behavior is established by the create request and the interactive start/attach connection.

## Inference for the S3 one-send premise

Assuming the S3 proof means “persist the durable one-send claim after `ExecCreate`, before any physical start,” an inert exec receipt can safely identify the intended command and make response-loss recovery idempotent only up to the start boundary. It does **not** provide a recoverable output stream.

The safe state machine is therefore:

`ExecCreate (receipt only) -> durable one-send claim -> exactly one ExecStart/ExecAttach action -> consume stream -> ExecInspect for terminal status`.

If the process crashes after a non-detached `ExecStart` request has reached Docker but before the response is observed, retrying `ExecStart` or calling `ExecAttach` is unsafe: both are another `/exec/{id}/start` POST and may attempt a second physical start. `ExecInspect` can distinguish `Running` versus exited and report an exit code, but cannot prove that missing output is empty or recover lost bytes. Therefore response-loss recovery can be “do not duplicate start; inspect and classify output as unavailable,” but cannot be “resume output exactly” with this protocol alone.

An outputful normal mode is possible only when the single physical start is the hijacked interactive request itself: call `ExecAttach` once after the durable claim, keep its connection alive, stream/demultiplex stdout and stderr, optionally write stdin, close it, then inspect. A prior inert/detached `ExecStart` followed by `ExecAttach` is not safe because attach is a second start request.

## Recommendation / blocker

**Recommendation:** keep the durable one-send seam at the operation that performs the one and only `/exec/{id}/start` POST. Add a normal-output variant whose physical action is `ExecAttach` (or an equivalent raw hijacked start request), with `TTY` and stream attachment fixed at `ExecCreate`. Do not model `ExecStart(detach=true)` as a receipt that can later be attached.

**Verified blocker:** if T19 requires all of the following simultaneously—(a) an inert receipt must be created and durably acknowledged before any start, (b) response-loss recovery must recover stdout/stderr bytes, and (c) no duplicate physical start may occur—Docker's pinned Exec API cannot satisfy it. Docker provides no output replay or attach-after-detached-start primitive, and `ExecAttach` is itself another start POST. The contract must either accept output-unavailable classification after ambiguous start, or move the durable claim/ack boundary to the single hijacked `ExecAttach` action and use an external durable output sink inside the command.

## Minimal interface and acceptance tests

These are research recommendations, not implemented tests:

- **Single-start spy:** fake Docker client records calls; normal-output execution must make exactly one create and exactly one start/attach POST. `ExecStart` followed by `ExecAttach` fails this test.
- **Create options:** assert stdin/stdout/stderr and TTY are selected at create time; reject console-size options when TTY is false (the pinned SDK returns `ErrInvalidArgument`).
- **Demultiplexing:** non-TTY frames with stream byte 1 go to stdout and byte 2 to stderr; TTY output is raw and must not be passed through `stdcopy`.
- **Input:** when stdin is enabled, the returned hijacked connection is the write path; closing stdin must be possible without closing output prematurely.
- **Response-loss classification:** after a start transport error, perform only `ExecInspect`; never retry start/attach. If inspect says running, classify as started/stream unavailable; if exited, report exit code with output-unavailable marker unless an external sink supplied bytes.
- **No fabricated output:** no recovery path may synthesize stdout/stderr from exit status, logs, or an inspect response.
- **Terminal status:** after stream EOF/connection close, inspect once for exit code; preserve stream errors separately from process exit status.

## Source pointers

- `go.mod:262-263` — pinned Moby API/client versions.
- `/Users/wuyongjun/go/pkg/mod/github.com/moby/moby/client@v0.5.1/container_exec.go:14-72` — create options and create request.
- Same file `:74-102` — `ExecStartOptions` and detached/non-stream start implementation.
- Same file `:104-151` — `ExecAttachOptions`, hijacked stream, and second `/exec/{id}/start` POST.
- Same file `:153-191` — inspect fields and lack of output replay.
- Official API docs, Exec Start and Stream details sections — two-stage semantics, detach behavior, TTY/raw versus non-TTY multiplexing.

