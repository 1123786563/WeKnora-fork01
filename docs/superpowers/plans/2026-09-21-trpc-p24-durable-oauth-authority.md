# Durable Native OAuth Authority Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist and verify native pending OAuth attempts across SQLite and PostgreSQL without enabling real provider dispatch.

**Architecture:** A durable authority repository owns immutable OAuth state binding and one-shot callback receipt consumption. A service adapter validates the receipt through an injected verifier and returns a redacted typed result. Provider HTTP/token exchange and all Runner/tool dispatch remain unavailable unless separately wired.

**Tech Stack:** Go, GORM, existing versioned SQLite/PostgreSQL migrations, testify, existing native pending seams.

**Spec:** `docs/adr/0014-native-pending-authority-and-reservation.md`; `docs/plans/2026-09-19-trpc-native-agent-p2-detailed.md` P2.4; `docs/plans/trpc-native/interfaces.md` pending/OAuth sections.

## Global Constraints

- No Runner, external tool dispatch, provider HTTP, token persistence, router, or container production wiring.
- OAuth attempts bind tenant, principal, service, redirect allowlist, expiry, pending key and receipt hash.
- SQLite and PostgreSQL are both acceptance targets; a missing PostgreSQL DSN is `NOT VERIFIED`, not pass.
- Callback receipt consumption is one-shot and idempotent only for the same verified receipt.

## Review Focus

- Callback for another tenant/principal/service/redirect never consumes state.
- Expired or replayed state never becomes authorized.
- Verifier failure leaves state unconsumed.
- Competing callbacks produce exactly one accepted receipt.
- Read APIs expose redacted metadata, never callback/token proof material.

---

### Task 1: OAuth attempt schema and immutable repository binding

**Files:** migrations for SQLite/PostgreSQL; `internal/application/repository/native_oauth.go`; `internal/application/repository/native_oauth_test.go`.

- [ ] Write failing SQLite and PostgreSQL tests for create/read attempt identity, expiry, scope mismatch, and atomic same-receipt replay.
- [ ] Add migrations with tenant/pending/state uniqueness, receipt hash, expiry, consumed revision/time, and no token material.
- [ ] Implement repository create/get/consume CAS APIs.
- [ ] Run SQLite and PostgreSQL focused tests, then commit.

### Task 2: Service authority adapter and receipt verifier port

**Files:** `internal/application/service/native_oauth.go`; `internal/application/service/native_oauth_test.go`; native pending contracts/tests as needed.

- [ ] Write failing tests for verifier rejection, redirect/service/principal mismatch, redacted output, and one-shot concurrency.
- [ ] Implement injected verifier port and typed redacted receipt result; default adapter stays unavailable.
- [ ] Run focused race tests and commit.

### Task 3: Cross-dialect acceptance and coordinator handoff

**Files:** `internal/application/repository/native_oauth_test.go`; `internal/application/service/native_oauth_test.go`; P2.4 coordinator tests.

- [ ] Add cancel-vs-callback and pending-resolution handoff tests for SQLite/PostgreSQL.
- [ ] Verify expiry/replay/mismatch leave pending unconsumed and make zero dispatch authorization.
- [ ] Run focused cross-dialect/race/vet/diff evidence and commit.
