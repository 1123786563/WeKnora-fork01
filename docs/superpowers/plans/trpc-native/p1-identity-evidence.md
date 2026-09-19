# P1 Track C identity and replay-cursor evidence

Date: 2026-09-19
Track: C / Task 3
Module: `github.com/Tencent/WeKnora/tools/trpc-native-p1/identityprobe`

## Scope and fixed inputs

This is an independent executable encoding specification. It is not product
wiring and it does not implement a trusted `ScopeResolver`. The nested module
pins `trpc.group/trpc-go/trpc-agent-go v1.11.0`; it contains no `replace`
directive and is run with `GOWORK=off`.

The implementation produces `session.Key` and `memory.UserKey` using the P0
namespace rules, plus checkpoint namespaces and replay cursors. Its acceptance
classification is **PASS for the local encoding contract only**.

## RED

Command, from `tools/trpc-native-p1/identityprobe`:

```sh
GOWORK=off go test ./... -run TestIdentity -count=1
```

Exit code: 1.

Actual behavior before `identity.go` existed: the compiler reported undefined
`Identity` and `SessionKey` in `identity_test.go`. This was the intended missing
production API failure for the first encoding test.

A later focused RED command also exposed an over-coupling bug before its fix:

```sh
GOWORK=off go test ./... -run TestKeysValidateOnlyTheirOwnOpaqueScopeFields -count=1
```

Exit code: 1. `SessionKey` rejected a blank `SubjectID` with `subject ID:
identity value must be nonblank valid UTF-8`. The contract requires Session and
Memory key validation to remain independent, so `SessionKey` was changed to
validate only tenant, owner, and session; `MemoryKey` validates only tenant and
subject.

## GREEN

Commands, from `tools/trpc-native-p1/identityprobe`:

```sh
GOWORK=off go test ./... -count=1
GOWORK=off go test -race -count=20 ./...
```

Both commands exited 0. The normal suite passed and the repeated race suite
passed twenty executions without a race report.

The tests cover tenant separation; independent owner and memory subject keys;
independent key-domain validation;
opaque Unicode and punctuation; delimiter-collision resistance; zero tenant;
empty, blank-only, and invalid UTF-8 values; `uint64` maximum tenant; canonical
checkpoint segments; `int64` maximum cursor; malformed, padded, and
noncanonical base64url; empty encoded runs; canonical base64url that decodes to
blank or invalid UTF-8 runs; blank and invalid UTF-8 expected runs; wrong
cursor version/part count/run; negative, signed, leading-zero, and overflow
sequence values.

## Module metadata observation

The Go module cache contains the pinned root module with these observed sums:

```text
trpc.group/trpc-go/trpc-agent-go v1.11.0
sum: h1:LwMxQwT2l6hqWUVARfVA/ef2tq8gJCzbImSHurpaPIo=
go.mod sum: h1:bIZcN4N9sGpA42sWfE98XCPl9ZgMi6fMDLGYOaXNe9A=
```

With `GOPROXY=https://proxy.golang.org,direct`, both `GOWORK=off go list -m
all` and `GOWORK=off go mod download -json trpc.group/trpc-go/trpc-agent-go@v1.11.0`
completed successfully and reported the same root module version and sums. The
normal shell's `GOPROXY=off` initially prevented those metadata commands, but
the cached, checksum-pinned module was sufficient for the test commands. This
metadata observation is not a database or authorization result.

## Limits

These tests do not prove current authentication, authorization, revocation,
tenant database isolation, session/memory persistence, cursor retention, or
SSE delivery. A decoded cursor is not an access token. P0's native product
execution NO-GO and the separate Session/Memory SQL evidence gates remain in
force.
