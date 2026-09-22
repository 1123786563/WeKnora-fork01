# Pass B Contract and Ownership Freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Freeze one owner for every Pass B legacy file and exception, define the public capability/event contracts that later plans consume, and make ownership/contract drift fail CI before B1 starts.

**Architecture:** B0 adds a dedicated `passbguard` that validates generated Pass B governance files against the accepted Pass A manifests and current Go tree. It resolves shared-host and Agent Runtime brief overlaps explicitly, records current external behavior and consumer call sites, and produces immutable B1 inputs without moving production business code.

**Tech Stack:** Go 1.26, Go AST/parser/types, YAML v3, Testify, existing `modulemove` and `architectureguard` tools.

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`; decomposition framework: `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md`

## Global Constraints

- B0 modifies governance documents, validation tools, tests, and module contract declarations only; it does not move legacy business implementations.
- Accepted baseline is main commit `78f18915f`; all counts compare against `docs/architecture/evidence/pass-a-acceptance.md`.
- Exactly 396 legacy entries and 105 import exceptions must have one child-plan owner each.
- No wildcard path, “first merged wins”, shared mutable owner, or duplicate claim is permitted.
- Existing route/worker/hook/migration counts remain 633, 23+23, 58, and 537.
- Contract records describe current behavior and current consumers; B0 does not invent new product semantics.
- Module façade and event changes after B0 require a serialized contract revision and architecture review.
- `cmd/desktop`, `docreader`, `client`, standalone SDKs, production SQL, and migrations remain untouched.

## Review Focus

- Generated ownership must fail when one file is omitted, duplicated, renamed on disk, or assigned to two child plans.
- Agent Runtime `native_archive`/`native_recovery` overlap must resolve deterministically and match both briefs after editing.
- Shared host packages must assign individual files, never whole directories, to child plans.
- Contract extraction must include every production consumer and reject an unrecorded new consumer.
- Event records must distinguish authoritative facts from commands and must require tenant, version, time, and idempotency metadata where applicable.

---

## File Structure

| File | Responsibility |
|---|---|
| `docs/architecture/passb/ownership-matrix.yaml` | Every legacy file/alias/exception, exact child-plan owner, destination, shared integration owner, and deletion barrier. |
| `docs/architecture/passb/contracts.yaml` | Frozen capability/route/worker/lifecycle contracts, current symbols, consumers, and required characterization tests. |
| `docs/architecture/passb/event-catalog.yaml` | Versioned authoritative events, producer, consumers, required metadata, ordering, and replay rule. |
| `docs/architecture/passb/exception-ledger.yaml` | All 105 temporary exceptions with removal owner and barrier. |
| `docs/architecture/passb/b0-evidence.md` | Counts, commands, decisions, test results, and review findings. |
| `tools/passbguard/model.go` | Strict YAML models and cross-file identifiers. |
| `tools/passbguard/load.go` | Known-fields loaders and path normalization. |
| `tools/passbguard/discover.go` | Reads manifests, aliases, imports, Go symbols, consumers, routes, workers, hooks, migrations. |
| `tools/passbguard/check.go` | Coverage, overlap, consumer, contract, event, exception, and baseline validations. |
| `tools/passbguard/main.go` | `go run ./tools/passbguard -root .` CLI. |
| `tools/passbguard/*_test.go` | Fixture and real-repository tests for every Review Focus item. |
| `Makefile` | Adds `check-passb-readiness`; existing targets remain unchanged. |

---

### Task B0.1: Add Strict Pass B Governance Models and Loader

**Files:**
- Create: `tools/passbguard/model.go`
- Create: `tools/passbguard/load.go`
- Test: `tools/passbguard/model_test.go`
- Test fixtures: `tools/passbguard/testdata/{valid,unknown-field,duplicate-id,wildcard-path}.yaml`

**Interfaces:**
- Produces: `LoadGovernance(root string) (*Governance, error)` and stable IDs `LegacyID`, `ContractID`, `EventID`, `ExceptionID`, `PlanID`.

- [ ] **Step 1: Write failing strict-loader tests**

```go
func TestLoadGovernanceRejectsUnknownField(t *testing.T) {
    _, err := LoadGovernance("testdata/unknown-field")
    require.ErrorContains(t, err, "field")
}

func TestLoadGovernanceRejectsWildcardPath(t *testing.T) {
    _, err := LoadGovernance("testdata/wildcard-path")
    require.ErrorContains(t, err, "exact repository path")
}
```

Also reject duplicate IDs, absolute paths, `..`, empty plan owners, and unknown module IDs.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./tools/passbguard -run TestLoadGovernance -count=1 -v`

Expected: FAIL because `LoadGovernance` does not exist.

- [ ] **Step 3: Implement strict schemas and loaders**

Use `yaml.Decoder.KnownFields(true)`. Define explicit structs:

```go
type LegacyOwnership struct {
    Path, Module, Plan, Destination, IntegrationOwner, DeleteBarrier string
}
type Contract struct {
    ID, Owner, Kind, Symbol, Signature, Stability string
    Consumers, CharacterizationTests []string
}
type Event struct {
    ID, Version, Producer, Meaning, Ordering, Replay string
    Consumers, RequiredMetadata []string
}
type Exception struct {
    ID, From, To, Plan, RemoveAt, Reason string
}
```

Normalize slash-separated repository paths and sort all slices by stable ID before validation/output.

- [ ] **Step 4: Run loader tests and commit**

Run: `go test ./tools/passbguard -run TestLoadGovernance -count=1`

Expected: PASS.

Commit: `test(passb): add strict governance schema loader`.

---

### Task B0.2: Generate and Validate the Ownership and Exception Ledgers

**Files:**
- Create: `tools/passbguard/discover.go`
- Create: `tools/passbguard/check.go`
- Test: `tools/passbguard/ownership_test.go`
- Create: `docs/architecture/passb/ownership-matrix.yaml`
- Create: `docs/architecture/passb/exception-ledger.yaml`

**Interfaces:**
- Consumes: 16 `docs/architecture/moves/*.yaml` manifests and architectureguard import exceptions.
- Produces: `DiscoverPassB(root string) (*Discovery, error)` and `CheckOwnership(g *Governance, d *Discovery) []Diagnostic`.

- [ ] **Step 1: Write failing coverage/overlap tests**

Fixtures must prove the checker reports exact diagnostics for one missing legacy file, duplicate owner, nonexistent path, owner/destination module mismatch, missing alias, unowned exception, and exception assigned after its dependent plan.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./tools/passbguard -run 'Ownership|Exception' -count=1 -v`

Expected: FAIL because discovery/checking functions do not exist.

- [ ] **Step 3: Implement manifest and repository discovery**

Reuse `modulemove.LoadManifestStrict`; enumerate `legacy_files`, `alias_obligations`, current `.go` paths, module imports, and exact architecture exceptions. Do not parse counts from prose evidence.

- [ ] **Step 4: Populate the ownership matrix**

Assign all 396 entries to the plan IDs in the framework. Required totals:

```text
B1 89   = identity 27 + airesource 33 + commercial 8 + execution 21
B2 150  = knowledge 84 + agentcatalog 55 + datasource 4 + appconnector 7
B3 102  = agentruntime 45 + conversation 44 + channels 7 + insights 6
B4 55   = workbench 19 + craft 30 + system 5 + policy 1
```

Every row names a concrete destination package and Integration Barrier.

- [ ] **Step 5: Populate the 105-entry exception ledger**

Copy exact `from`/`to` paths from the architecture guard source, assign one removal plan and IB/B5 deadline, and record the reason. No exception may be owned only by B5 when an earlier module plan can remove it.

- [ ] **Step 6: Run real-repository checks and commit**

Run: `go test ./tools/passbguard -run 'Ownership|Exception|RealRepo' -count=1`

Expected: PASS with `legacy=396`, `exceptions=105`, `overlaps=0`, `missing=0`; the alias total must equal discovery's alias-obligation count.

Commit: `docs(passb): freeze legacy and exception ownership`.

---

### Task B0.3: Resolve Shared-Host and Agent Runtime Brief Overlaps

**Files:**
- Modify: `docs/architecture/passb/agentruntime-engine.md`
- Modify: `docs/architecture/passb/agentruntime-protocol.md`
- Modify: `docs/architecture/passb/{conversation-session,conversation-queryhistory,knowledge-wikifaq,workbench,craft}.md`
- Modify: `docs/architecture/passb/ownership-matrix.yaml`
- Test: `tools/passbguard/overlap_test.go`

**Interfaces:**
- Produces: zero ambiguous phrases and exact owner for every file in `internal/handler/session`, `internal/application/repository`, `internal/application/service`, and Agent Runtime native families.

- [ ] **Step 1: Write failing ambiguity tests**

Reject governance/brief text containing `first merged wins`, `先合并者`, `二选一执行`, or two plan IDs claiming the same repository path.

- [ ] **Step 2: Apply the Agent Runtime ruling**

Record these exact owners:

- `native_archive.go` service and `handler/session/native_archive.go` → `34-agentruntime-protocol`;
- `native_recovery.go` plus native repository state/lease/pending/usage files → `33-agentruntime-engine`;
- pure native/nativecontract/nativeprobe/trpc/opencode/recoverytest packages → protocol plan;
- approval, run/attempt, checkpoint, decisions, events, inputs, lifecycle, tools journal → engine plan.

- [ ] **Step 3: Apply shared-host rulings**

- `wiki_fixer_scope.go` → Knowledge Wiki/FAQ;
- Workbench `workbench_*`/artifact files → Workbench;
- Craft `craft*` files → Craft;
- Agent Run/stream files → Agent Runtime Engine;
- remaining Session/Message/Feedback/share/attachment/stream files → Conversation Session;
- generic pagination/upload-limit/error helpers remain Platform until a separate consumer extraction proves otherwise.

- [ ] **Step 4: Apply housekeeping ruling**

`knowledge_housekeeping.go` and its business sweep rules remain Knowledge; System owns scheduling/lifecycle invocation through a narrow `KnowledgeHousekeeping` port. No duplicated sweep implementation.

- [ ] **Step 5: Run overlap tests and commit**

Run: `go test ./tools/passbguard -run 'Overlap|Ambiguous|RealRepo' -count=1`

Expected: PASS with zero overlap and zero ambiguous brief markers.

Commit: `docs(passb): resolve shared-host ownership`.

---

### Task B0.4: Freeze Capability and Composition Contracts

**Files:**
- Create: `docs/architecture/passb/contracts.yaml`
- Test: `tools/passbguard/contracts_test.go`
- Modify: `tools/passbguard/discover.go`
- Modify: `tools/passbguard/check.go`
- Modify: `internal/modules/*/module.go` comments only when they contradict the frozen registry counts; no implementation added

**Interfaces:**
- Produces: exact current symbol/signature/consumer/test record for every B1–B4 dependency and module composition surface.

- [ ] **Step 1: Write failing symbol/consumer discovery tests**

Use `go/parser` and `go/types` to extract exported function/interface signatures and production import/call consumers. Tests must detect missing symbol, changed signature, new unrecorded consumer, and consumer importing `/adapters` or another module's non-public subpackage.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./tools/passbguard -run Contract -count=1 -v`

Expected: FAIL because contract discovery is not implemented.

- [ ] **Step 3: Implement contract discovery and validation**

Each contract record has one of these kinds: `module-construction`, `capability-port`, `route-set`, `worker-set`, `lifecycle-set`, `wire-protocol`, `data-ownership`.

- [ ] **Step 4: Freeze B1 contracts from current behavior**

Record current symbols and consumers for:

- Identity actor/tenant/member/RBAC/audit decisions;
- AI Resource model/MCP/search/vector/storage resolution;
- Commercial admission/reservation/usage/payment/entitlement;
- Execution sandbox/target/workspace/terminal/browser lifecycle.

Each record names characterization tests already present; if none exists, add the missing characterization test path to the owning B1 plan's required input rather than inventing behavior in B0.

- [ ] **Step 5: Freeze downstream contracts**

Record Knowledge retrieval/ingest/worker sets, Conversation session/turn/stream/query-history sets, Agent Catalog immutable version/capability/install sets, Agent Runtime run/tool/approval/recovery wire sets, and Task/Timeline/Artifact sets.

- [ ] **Step 6: Freeze composition counts and module façade shape**

For each module record expected route entry points, worker task types, lifecycle hooks, and façade operations `NewModule`, `RegisterRoutes`, `RegisterWorkers`, `Start`, `Stop`. Absence of an operation is explicit, not inferred.

- [ ] **Step 7: Run contract checks and commit**

Run: `go test ./tools/passbguard -run 'Contract|RealRepo' -count=1`

Expected: PASS with zero missing symbols/consumers and baseline counts 633/23+23/58.

Commit: `docs(passb): freeze module capability contracts`.

---

### Task B0.5: Freeze the Versioned Event Catalog

**Files:**
- Create: `docs/architecture/passb/event-catalog.yaml`
- Test: `tools/passbguard/events_test.go`

**Interfaces:**
- Produces: authoritative event IDs consumed by Workbench, Insights, Commercial, Channels, Conversation, Agent Runtime, Knowledge, and Craft plans.

- [ ] **Step 1: Write failing event-schema tests**

Require `id`, positive integer `version`, producer, meaning, consumers, ordering, replay rule, idempotency key, and required metadata. Reject events named as imperative commands and duplicate producer/version pairs.

- [ ] **Step 2: Define the event families**

Create version-1 records for current observable facts:

- Agent Run started/attention/completed/failed/cancelled and tool/approval result;
- Conversation message/turn appended and session lifecycle;
- Knowledge processing/index/deletion completed or failed;
- Craft run/interaction/artifact/scheduled-run lifecycle;
- Usage fact observed and settlement outcome;
- Workbench task/timeline/attention/artifact projection updates;
- Notification delivery outcome and Channel delivery outcome.

Every tenant-scoped event requires `tenant_id`, `occurred_at`, `event_id`, `idempotency_key`, and actor/system origin. Events are facts; imperative operations remain synchronous commands/ports.

- [ ] **Step 3: Map current producers and consumers**

Record existing event bus, callbacks, repository projections, and notification paths. When no durable event exists today, mark `transport: in_process` and `replay: source-query`; B0 does not add a new broker.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./tools/passbguard -run Event -count=1`

Expected: PASS with no command-like event name and no missing tenant/idempotency metadata.

Commit: `docs(passb): freeze versioned event catalog`.

---

### Task B0.6: Add the Readiness Command and Produce B0 Evidence

**Files:**
- Create: `tools/passbguard/main.go`
- Test: `tools/passbguard/main_test.go`
- Modify: `Makefile`
- Create: `docs/architecture/passb/b0-evidence.md`
- Modify: `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` only to record B0 completion SHA/status

**Interfaces:**
- Produces: `make check-passb-readiness`; approved inputs for B1 detailed plans.

- [ ] **Step 1: Write failing CLI tests**

Assert success prints stable totals and exit 0; any diagnostic prints sorted `check: path: message` lines to stderr and exits 1.

- [ ] **Step 2: Implement CLI and Make target**

```make
.PHONY: check-passb-readiness
check-passb-readiness:
	go run ./tools/passbguard -root .
```

Success output is produced by this exact statement:

```go
fmt.Printf("pass-b readiness: legacy=%d aliases=%d exceptions=%d contracts=%d events=%d overlaps=%d missing=%d\n",
    report.Legacy, report.Aliases, report.Exceptions, report.Contracts,
    report.Events, report.Overlaps, report.Missing)
```

The CLI test asserts legacy is 396, exceptions is 105, overlaps/missing are zero, and aliases/contracts/events equal the independently discovered sets.

- [ ] **Step 3: Run focused and existing guard suites**

```bash
go test ./tools/passbguard ./tools/modulemove ./tools/architectureguard ./internal/bootstrap -count=1
make verify-module-moves
make check-backend-architecture
make check-passb-readiness
```

Expected: PASS; counts 633/23+23/58/537 unchanged.

- [ ] **Step 4: Run repository verification**

```bash
go build ./...
go test ./internal/... -count=1 -timeout=25m
golangci-lint run --new-from-rev=78f18915f ./...
git diff --check 78f18915f...HEAD
git diff 78f18915f...HEAD -- cmd/desktop docreader client
```

Expected: build/tests/guards PASS or match only documented Pass A debt; excluded-scope diff is empty.

- [ ] **Step 5: Write evidence and request architecture review**

Record command outputs, counts, all rulings, remaining known debt, and the exact SHA B1 plans must start from. Reviewer checks Spec compliance, 396/105 coverage, contract consumer completeness, event semantics, and no production behavior change.

- [ ] **Step 6: Commit B0**

Commit: `docs(passb): complete contract and ownership freeze`.

---

## Plan Self-Review Record

- **Spec coverage:** Ownership, contracts, events, exceptions, overlap rulings, readiness command, evidence, and B1 inputs are all assigned to B0.1–B0.6.
- **Completeness:** Contract/event totals are discovered and cross-checked by the guard; their schemas, required families, validation rules, and success format are exact.
- **Type consistency:** Governance structs and stable IDs are defined in B0.1 and consumed unchanged by discovery/check/CLI tasks.
- **Review Focus coverage:** omitted/duplicate owners B0.2, runtime overlap B0.3, consumer drift B0.4, event semantics B0.5, and final zero-diagnostic readiness B0.6.
- **Execution:** Tasks are serial because each produces the facts consumed by the next; use one implementation worktree with a fresh reviewer after every task and a final architecture review.
