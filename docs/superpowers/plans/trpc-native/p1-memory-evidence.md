# P1 native Memory SQL evidence

Status: **incompatible for P1 governance; SQL behavior characterized.**

The probe is [memoryprobe](../../../../tools/trpc-native-p1/memoryprobe/). It
uses the exact standalone module requirements `trpc-agent-go@v1.11.0`,
`memory/sqlite@v1.11.0`, and `memory/postgres@v1.11.0`, plus the exact
`storage/postgres@v1.11.0` selected to avoid the backend module's development
pseudo-version. No root module, application code, migration, or SDK cache was
modified.

| Capability | SQLite | PostgreSQL | Classification |
| --- | --- | --- | --- |
| AppName/UserID isolation, reopen, and clear | pass | pass | PASS for tested SQL behavior |
| Same content add, metadata, update, delete, clear | pass | pass | PASS for tested SQL behavior |
| 20 concurrent same-content adds | pass | pass | PASS for tested SQL behavior |
| `Close` owns supplied SQLite `*sql.DB` | pass | n/a: backend creates its own client | PASS for documented ownership |
| stale extraction after delete or clear cannot resurrect content | raw service resurrects | raw service resurrects | **incompatible / extension-required** |

Commands run from `tools/trpc-native-p1/memoryprobe`:

```sh
GOWORK=off GOPROXY=https://proxy.golang.org,direct go mod tidy
GOWORK=off GOPROXY=https://proxy.golang.org,direct go list -m -json trpc.group/trpc-go/trpc-agent-go trpc.group/trpc-go/trpc-agent-go/memory/sqlite trpc.group/trpc-go/trpc-agent-go/memory/postgres trpc.group/trpc-go/trpc-agent-go/storage/postgres
GOWORK=off GOPROXY=https://proxy.golang.org,direct go mod download -json trpc.group/trpc-go/trpc-agent-go@v1.11.0 trpc.group/trpc-go/trpc-agent-go/memory/sqlite@v1.11.0 trpc.group/trpc-go/trpc-agent-go/memory/postgres@v1.11.0 trpc.group/trpc-go/trpc-agent-go/storage/postgres@v1.11.0
GOWORK=off GOPROXY=https://proxy.golang.org,direct go test -run TestMemory -count=1 -v ./...
P1_MEMORY_PG_DSN='postgres://postgres@127.0.0.1:32768/postgres?sslmode=disable' GOWORK=off GOPROXY=https://proxy.golang.org,direct go test -run TestMemory -count=1 -v ./...
P1_MEMORY_PG_DSN='postgres://postgres@127.0.0.1:32768/postgres?sslmode=disable' GOWORK=off GOPROXY=https://proxy.golang.org,direct go test -race -count=20 -timeout=5m ./...
```

The exact module download sums were root
`h1:LwMxQwT2l6hqWUVARfVA/ef2tq8gJCzbImSHurpaPIo=`, SQLite
`h1:Xb5/BLdIhS0FPEdqDdtq5Mm3I3wQaytFYiGq+F24ln8=`, PostgreSQL
`h1:l5osN13fRX8oj3luKydVbxd/s+VKXwDd5RN1DkT/w2g=`, and required storage
`h1:t7sovgCKIVbxNETujKGvJ4hWZlQY+hoFmE2dNeT9piY=`. The non-contract commands
exited `0`; the final race run took 12.016s. PostgreSQL used a fresh `p1m_*`
schema per subtest and cleanup issued `DROP SCHEMA ... CASCADE`.
The normal suite skips `TestMemoryContractRejectsStaleExtractionAfterClear`;
that skip prevents a characterization result from being misreported as a
governance pass. With `P1_REQUIRE_CONTRACT=1`, this test is intentionally red:
raw `AddMemory` has no generation/tombstone/CAS parameter and returns nil after
clear, so old extraction output can reappear.

The native `EnqueueAutoMemoryJob` only enqueues into an in-process worker when
an extractor is configured. Its eventual writes call the same `AddMemory`
method, and neither the enqueue nor write interface accepts an authorization
revision, setting generation, tombstone, or conditional expected value. A
product adapter must recheck current authorization and settings and make the
generation/tombstone comparison and write atomic at commit time.

Existing WeKnora memory functionality is broader than the SDK metadata. Its
`MemoryItem` retains topic, normalized contradiction key, importance, origin,
status, source session/message provenance, validity/expiry, replacement and
usage fields. `MemorySettings` merges workspace/user enablement, write mode,
capacity, extraction scheduling/instructions, and vector/retrieval settings.
The existing repository also records deletion tombstones and extraction lease
state. These are required extension inputs for P1 and must not be discarded in
an adapter; most importantly, the persisted invalidation generation/tombstone
and authorization recheck must guard delayed extraction commits.
