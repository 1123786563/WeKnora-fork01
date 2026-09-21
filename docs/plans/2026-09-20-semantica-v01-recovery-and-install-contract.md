# Semantica V01 Recovery and Installed-Distribution Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recover the deleted Semantica planning facts as explicitly non-authoritative history, then establish a reproducible, minimal, *installed-distribution* V01 contract for the selected Semantica candidate.

**Architecture:** ADR-0002 and the 2026-09-20 rebaseline remain binding. Historical 2026-09-11 files supply the deleted 24-task decomposition and original design context only; each must carry a prominent authority notice. V01 owns an isolated `semantic/experiments` uv project that installs the exact pinned Semantica distribution, proves required public imports and runtime signatures, and emits machine-readable evidence without attempting production services, storage, models, credentials, or promotion.

**Tech Stack:** Python 3.12.12, uv, pytest, `importlib.metadata`, `inspect`, SHA-256, Semantica candidate `0.6.8`; Markdown and JSON evidence.

**Spec:** [ADR-0002](../adr/0002-semantica-independent-service.md), [2026-09-20 rebaseline](2026-09-20-semantica-rebaseline.md), and the recovered historical [2026-09-11 design](../specs/2026-09-11-semantica-graphrag-reasoning-design.md).

## Global Constraints

- ADR-0002 and rebaseline supersede all unverified historical proposals.
- Keep V01 `pending` until a current, actual installed-distribution run records exact commands, exit codes, baseline SHA, lock hash, environment, artifact path, limitations, and evidence layer.
- Freeze the candidate by exact release `semantica==0.6.8` and the public wheel SHA-256 `0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7`; reject an installation resolving a different distribution or version.
- Test imports must cover `NERExtractor`, `RelationExtractor`, `ContextGraph`, `ContextRetriever`, `Reasoner`, `GraphReasoner`, and `GraphStore`; evidence must record runtime `inspect.signature` output for every callable tested.
- Never treat source text inspection, zip extraction via `PYTHONPATH`, mocks, or a successful test of the probe code as V01 acceptance.
- The experiment may not use provider credentials, model calls, persistent graph/vector stores, Go authorization, production configuration, DocReader environments, or external service deployment.
- Preserve existing 2026-09-20 bounded-probe artifacts as lower-layer evidence; do not overwrite them.

## Review Focus

- A package with matching import names but a different installed version or artifact must fail the verifier before capability output claims success.
- A missing callable or changed runtime signature must produce a nonzero verifier exit and retained failure evidence, rather than a boolean-only result.
- An interpreter or environment that succeeds only through `PYTHONPATH` wheel extraction must not be accepted as an installed-distribution run.
- Re-running from a freshly created uv environment must not require or import `docreader/.venv`.
- Broken or moved evidence paths must be detected by tests or verifier output; generated evidence must point beneath `docs/plans/semantica/evidence/`.

### Task 1: Recover historical planning references with authority boundaries

**Files:**
- Create: `docs/plans/2026-09-11-semantica-implementation.md`
- Create: `docs/plans/2026-09-11-semantica-00-verification.md` through `docs/plans/2026-09-11-semantica-06-rollout.md`
- Create: `docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md`
- Modify: `docs/plans/semantica/progress.md`
- Test: Git path/content checks executed from the worktree

**Interfaces:**
- Consumes: historical files from `cc424e71097a4f29bd71a783964b9e4cdd6255c4^` and the binding ADR/rebaseline.
- Produces: reachable linked references for the 24-task decomposition; all readers can determine that ADR/rebaseline win on conflict.

- [ ] **Step 1: Write the failing reference-integrity check**

```sh
test -f docs/plans/2026-09-11-semantica-implementation.md
test -f docs/plans/2026-09-11-semantica-00-verification.md
test -f docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md
rg -n 'ADR-0002.*rebaseline.*优先|rebaseline.*ADR-0002.*优先' \
  docs/plans/2026-09-11-semantica-implementation.md \
  docs/plans/2026-09-11-semantica-00-verification.md \
  docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md
```

- [ ] **Step 2: Run the check and verify RED**

Run: the commands above.

Expected: `test -f` fails because the historical files are absent from the current checkout; this is a documentation-recovery failure, not an application test failure.

- [ ] **Step 3: Restore the exact historical sources and add the authority notice**

Use `git show cc424e71097a4f29bd71a783964b9e4cdd6255c4^:<historical-path>` as the only source material. Restore the one total plan, seven subplans, and original spec at their current `docs/plans/` and `docs/specs/` paths. Prepend each restored document with this notice, translated only when its surrounding document language requires it:

```markdown
> **Historical planning reference — not an implementation authority.**
> [ADR-0002](../adr/0002-semantica-independent-service.md) and the
> [2026-09-20 Semantica rebaseline](../plans/2026-09-20-semantica-rebaseline.md)
> govern any conflict, especially unverified versions, protocol, storage,
> topology, thresholds, and promotion decisions.
```

Correct relative paths per containing directory. Repair `progress.md` links so its total-plan link resolves to `../2026-09-11-semantica-implementation.md`; retain its statement that all 24 formal tasks are pending.

- [ ] **Step 4: Run reference-integrity checks and link scan**

Run:

```sh
for file in docs/plans/2026-09-11-semantica-implementation.md docs/plans/2026-09-11-semantica-0{0,1,2,3,4,5,6}-*.md docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md; do test -f "$file"; done
rg -n 'Historical planning reference|ADR-0002.*rebaseline|rebaseline.*ADR-0002' docs/plans/2026-09-11-semantica-* docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md
rg -n '\]\(\.\./2026-09-11-semantica-implementation\.md\)' docs/plans/semantica/progress.md
```

Expected: exit 0. The recovered files are not evidence that any V01–O03 task passed.

- [ ] **Step 5: Commit the scoped recovery**

```sh
git add docs/plans/2026-09-11-semantica-*.md docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md docs/plans/semantica/progress.md
git commit -m "docs(semantic): recover historical planning references"
```

### Task 2: Establish V01 installed-distribution verification

**Files:**
- Create: `semantic/experiments/pyproject.toml`
- Create: `semantic/experiments/uv.lock`
- Create: `semantic/experiments/test_import_contract.py`
- Create: `semantic/experiments/verify_version.py`
- Create: `docs/plans/semantica/evidence/<run-date>/v01-installed-contract.json`
- Modify: `semantic/experiments/README.md`
- Modify: `docs/plans/semantica/progress.md`

**Interfaces:**
- Consumes: the exact installed distribution metadata and `uv.lock` bytes.
- Produces: verifier JSON schema version 1 with `python`, `distribution`, `lock_hash`, `command`, `exit_code`, `capabilities`, `limitations`, and an explicit `actual-runtime` evidence layer.

- [ ] **Step 1: Write failing import-contract tests**

```python
def test_required_import_contract():
    from semantica.semantic_extract import NERExtractor, RelationExtractor
    from semantica.context import ContextGraph, ContextRetriever
    from semantica.reasoning import Reasoner, GraphReasoner
    from semantica.graph_store import GraphStore

    assert all(callable(item) for item in (
        NERExtractor, RelationExtractor, ContextGraph, ContextRetriever,
        Reasoner, GraphReasoner, GraphStore,
    ))

def test_installed_distribution_is_the_frozen_candidate():
    from importlib.metadata import version
    assert version("semantica") == "0.6.8"
```

Add tests that load verifier output from a temporary output path and assert it records each callable's nonempty runtime signature, installed distribution version, and the project lock SHA-256.

- [ ] **Step 2: Run RED with a valid test environment**

Run:

```sh
uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q
```

Expected: tests fail because the isolated project, installed candidate, and verifier do not yet exist. If uv cannot create the Python 3.12.12 environment, capture that environmental error separately and do not count it as the behavioral RED.

- [ ] **Step 3: Add the minimal project and verifier implementation**

Set `requires-python = "==3.12.*"` and pin `semantica==0.6.8` plus only pytest as the test dependency. Generate and commit `uv.lock` from the resolved project. Implement the verifier with `importlib.metadata.version`, `importlib.metadata.distribution`, `inspect.signature`, and SHA-256 of the committed lock. It must assert the installed distribution is version 0.6.8, its metadata locates the installed package rather than a temporary extracted wheel, and it must write structured failure evidence before returning nonzero.

Represent every capability as:

```json
{
  "status": "available",
  "signature": "<runtime inspect.signature output>",
  "evidence_layer": "actual-runtime"
}
```

On import, version, signature, lock, or path failure, use `status: "unavailable"`, preserve the error reason, and exit nonzero. Do not invoke constructors, providers, storage clients, or model calls.

- [ ] **Step 4: Verify GREEN and clean-environment replay**

Run:

```sh
uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q
uv run --isolated --project semantic/experiments python semantic/experiments/verify_version.py --output docs/plans/semantica/evidence/<run-date>/v01-installed-contract.json
```

Expected: both exit 0; the JSON identifies CPython, platform, exact Semantica distribution metadata, lock hash, each required import and runtime signature, exact invoked command and exit code, and limitations. Confirm `semantica.__file__` is under the uv project environment and not `/tmp` or `docreader/.venv`.

- [ ] **Step 5: Update durable instructions and ledger without overclaiming**

Document the exact clean replay command, generated artifact location, and known limits in `README.md`. Add a dated V01 execution row to `progress.md` containing baseline SHA, dirty-tree condition, lock hash, commands and exit codes, Python/platform, artifact path, evidence layer, review result, commit SHA, and limitations. Only set V01 to `implemented` or `verified` if the ledger's required evidence is actually present; otherwise leave it `pending`/`blocked` with the concrete reason.

- [ ] **Step 6: Commit the scoped V01 contract**

```sh
git add semantic/experiments/pyproject.toml semantic/experiments/uv.lock semantic/experiments/test_import_contract.py semantic/experiments/verify_version.py semantic/experiments/README.md docs/plans/semantica/evidence docs/plans/semantica/progress.md
git commit -m "feat(semantic): establish V01 installed-distribution contract"
```

## Self-Review

- Spec coverage: Task 1 restores the detailed execution/spec references while retaining ADR/rebaseline authority; Task 2 covers the rebaseline V01 requirement for a frozen candidate, actual installation, runtime imports/signatures, traceable evidence, and no production capability claim.
- Deliberate non-goals: V02 storage/reasoning, V03 quality, protocol C01, models, deployment, and backend promotion remain out of scope and must remain pending.
- Interface consistency: Task 2 produces a standalone JSON artifact and does not create production Python imports; downstream tasks must consume evidence rather than infer APIs from prose.
- Placeholder scan: no implementation step relies on unspecified API names or an unbounded candidate version.
