# Craft #107 T19 Normal Output Provider Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close early EOF false completeness, blocked output sink lifetime and typed callback panic findings from the normal-output provider Task 1 independent review.

**Architecture:** Treat stream EOF as provisional until exact-ID terminal observation; freeze returned stream state on cancellation and prohibit late callback or output append by a cancellation-aware sink/lifecycle contract. Convert callback panics into typed partial errors.

**Tech Stack:** Go, pinned Moby client, bounded demux worker, targeted race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-normal-output-task1-review.md`; `docs/plans/2026-09-24-craft-107-t19-normal-output-architecture.md`.

## Global Constraints

- Exactly one hijacked `ExecAttach` after durable claim; no detached Start, second attach, or legacy Exec fallback.
- Output is complete only with authoritative terminal process state and fully persisted stream. Missing bytes, failed sink/callback, cancellation or deadline remain partial/unavailable with no false build success.
- A caller supplied sink cannot mutate durable output after the provider returns a frozen result. If this cannot be guaranteed for an arbitrary sink, require a typed cancellation/seal contract and keep unsafe sinks unsupported.
- No production routing, application S2 coordinator changes, commit or push; exact checkpoint and independent review.

## Review Focus

- Frame-boundary EOF while exact-ID inspect says Running must remain partial/unknown.
- Cancellation while the sink is blocked must not race result fields or deliver a callback after return.
- A late sink unblock after timeout cannot append committed output behind a frozen result.
- Typed callback panic returns a partial error without crashing the host process.
- Normal terminal EOF, demux, stdin half-close and marker-once live behavior remain intact.

---

### Task 1: Freeze output lifecycle and terminal completeness

**Depends on:** Normal-output provider Task 1 reviewed FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** `internal/modules/execution/sandbox/docker_normal_exec.go`, `_test.go` only.

**Consumes / produces:** Existing one-attach provider and typed stream outcome; produces terminal-gated completeness and bounded, race-free cancellation of output persistence/callbacks.

- [ ] Capture preimage. RED EOF-while-running, blocked sink cancellation followed by late release under `-race`, and typed callback panic. Verify the original wrong complete/panic/race outcomes.
- [ ] Gate `TransportComplete` on exact-ID observed terminal state after clean stream EOF. If Running/Unknown, classify partial/unknown even if frame parsing ended cleanly.
- [ ] Replace unsynchronized writer result reads with a frozen synchronized snapshot. Define and enforce a context-aware or sealable sink contract so no append/callback can occur after return; close stream, cancel sink, drain within bound, and reject unsupported unbounded sinks rather than pretend they are safe.
- [ ] Recover typed callback panics into partial errors, preserving the one-send receipt and no retry.
- [ ] Run focused normal and race tests, compile/diff/gofmt, and rerun bounded real Docker proof if transport behavior changed. Save exact checkpoint/report/patch and independent review.

**Acceptance / failure handling:** F1–F3 closed without a second physical start or fabricated output. If a blocked arbitrary sink cannot meet bounded cancellation, report exact interface limitation and keep normal-output Task2 gated.
