# Semantica V03 Offline Evaluation Foundation Implementation Plan

> **For agentic workers:** REQUIRED WORKFLOW: Use the dispatching-parallel-agents workflow to keep V03 isolated from the independent A01 stream. Follow RED → GREEN for each task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a deterministic offline scorer and synthetic Chinese evaluation set that can later ingest real native/Semantica observations without inventing quality, latency, cost, or launch-threshold evidence.

**Architecture:** The evaluator consumes immutable case JSONL plus separately produced observation JSONL; it never calls a model, database, native retriever, or Semantica service. It emits per-case/source metrics and aggregates while labeling each observation by its evidence layer. A synthetic example is used only for parser/scorer smoke coverage. The acceptance policy remains unapproved with no numeric thresholds until comparable authorized runs exist and a product owner confirms them.

**Tech Stack:** Python 3.12, existing locked `semantic/experiments` environment, standard library only, pytest.

**Spec:** `docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md` §§5, 8, 13–14; `docs/adr/0002-semantica-independent-service.md`; `docs/plans/2026-09-20-semantica-rebaseline.md` (Q5, Q11, Q14, Q17 and evidence gates); V03 in `docs/plans/2026-09-11-semantica-00-verification.md`.

## Global Constraints

- V02 is verified only for its dedicated local Neo4j real-storage topology; this plan does not claim the shared-isolated comparison, production topology, or query capability.
- The V02 controlled graph may seed synthetic test cases, but every case and observation in this plan must be labeled synthetic; synthetic scores are not native-vs-Semantica quality evidence.
- Keep evidence precision/recall and permission leakage as separate metrics; a permission leak is a hard failure and cannot be averaged away.
- Keep `acceptance-policy.json` at `approved: false`; do not write numeric thresholds without measured, comparable runs and explicit product confirmation.
- No network access, native backend, model/provider credentials, live-model call, production KB data, production switch, or model/budget claim.
- `semantic/experiments/pyproject.toml` pins Python to `3.12.*` and Semantica to `0.6.8`; do not edit the V01/V02 lock for this standard-library-only evaluator.
- Rebaseline requires `controlled-provider` and `live-model` evidence to remain distinct; the existing mocked GraphReasoner seam is not controlled-provider evidence.

## Review Focus

- Unsupported or forbidden evidence appears: report a permission leak independently and fail the hard gate even when other scores are high.
- Empty expected evidence: unanswerable cases score recall as 1 only when no evidence/conclusion is returned; empty predictions for answerable cases must not receive perfect precision.
- Duplicate/missing case observations: reject ambiguous or incomplete input rather than silently dropping cases.
- Requested mode differs from actual mode: record the downgrade/escalation and mark the mode gate false; GraphRAG cannot silently become Reason.
- Missing latency/token/model fields: preserve them as unavailable/null; never coerce missing usage to zero or treat synthetic values as runtime measurements.

---

### Task 1: Define scorer contract with failing behavior tests

**Files:**
- Create: `semantic/experiments/test_evaluate.py`
- Create: `semantic/experiments/evaluate.py`

**Interfaces:**
- `score_case(case: dict[str, object], observation: dict[str, object]) -> dict[str, object]` returns `source_precision`, `source_recall`, `correct`, `unanswerable_correct`, `permission_leak`, `mode_match`, and `hard_gate_pass`.
- `aggregate_cases(rows: list[dict[str, object]]) -> dict[str, object]` returns case count, correctness rate, mean source precision/recall, unanswerable accuracy, hard-gate failures, p50/p95 latency, and total reported input/output tokens. Missing measurements remain `None` and are omitted from aggregate denominators.
- `load_cases(path: Path) -> list[dict[str, object]]` requires `case_id`, `scenario`, `synthetic`, `tenant_id`, `kb_id`, `document_revisions`, `query`, `requested_mode`, `expected_evidence_ids`, `expected_conclusion_ids`, `answerable`, and `forbidden_evidence_ids`.
- `load_observations(path: Path) -> list[dict[str, object]]` requires unique `case_id`, `backend`, `requested_mode`, `actual_mode`, `actual_status`, `actual_evidence_ids`, `actual_conclusion_ids`, `access_violations`, and `evidence_layer`. Optional `latency_ms`, `input_tokens`, `output_tokens`, `engine_version`, and `model_version` remain null when unavailable.
- `evaluate_run(cases, observations) -> dict[str, object]` rejects duplicate/missing/extra observation IDs and emits schema version, evidence layer, synthetic-only flag, per-case scores, aggregate metrics, and hard-gate status.

The test imports `Path`, `json`, `pytest`, and `score_case`/`aggregate_cases` from `evaluate.py`. Task 3 defines `FIXTURES = Path(__file__).parent / "fixtures"`; Task 4 defines `POLICY = Path(__file__).resolve().parents[2] / "docs/plans/semantica/acceptance-policy.json"` and reads that exact file.

- [ ] **Step 1: Write tests for evidence precision, recall, and empty sets**

```python
def test_score_case_penalizes_unsupported_evidence():
    case = {"expected_evidence_ids": ["e1"], "expected_conclusion_ids": ["c1"], "answerable": True, "forbidden_evidence_ids": []}
    observation = {"actual_evidence_ids": ["e1", "e-hidden"], "actual_conclusion_ids": ["c1"], "actual_status": "answered", "requested_mode": "reason", "actual_mode": "reason"}
    score = score_case(case, observation)
    assert score["source_precision"] == 0.5
    assert score["source_recall"] == 1.0
    assert score["correct"] is True

def test_empty_prediction_does_not_score_as_perfect_for_answerable_case():
    case = {"expected_evidence_ids": ["e1"], "expected_conclusion_ids": ["c1"], "answerable": True, "forbidden_evidence_ids": []}
    observation = {"actual_evidence_ids": [], "actual_conclusion_ids": [], "actual_status": "insufficient_evidence", "requested_mode": "reason", "actual_mode": "reason"}
    score = score_case(case, observation)
    assert score["source_precision"] == 0.0
    assert score["source_recall"] == 0.0
    assert score["correct"] is False

def test_unanswerable_requires_no_claim_and_no_evidence():
    case = {"expected_evidence_ids": [], "expected_conclusion_ids": [], "answerable": False, "forbidden_evidence_ids": []}
    observation = {"actual_evidence_ids": [], "actual_conclusion_ids": [], "actual_status": "insufficient_evidence", "requested_mode": "reason", "actual_mode": "reason"}
    assert score_case(case, observation)["unanswerable_correct"] is True
```

- [ ] **Step 2: Confirm RED**

Run: `uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q`

Expected: collection fails because `evaluate.score_case` is not implemented; this is the intended missing-behavior RED, not an environment failure.

### Task 2: Implement pure scorer, hard gates, and aggregates

**Files:**
- Create: `semantic/experiments/evaluate.py`
- Modify: `semantic/experiments/test_evaluate.py`

- [ ] **Step 1: Add permission leak and mode mismatch tests**

```python
def test_permission_leak_is_hard_gate():
    case = {"expected_evidence_ids": ["e1"], "expected_conclusion_ids": ["c1"], "answerable": True, "forbidden_evidence_ids": ["e-forbidden"]}
    observation = {"actual_evidence_ids": ["e1", "e-forbidden"], "actual_conclusion_ids": ["c1"], "actual_status": "answered", "requested_mode": "graph_rag", "actual_mode": "reason"}
    score = score_case(case, observation)
    assert score["permission_leak"] is True
    assert score["mode_match"] is False
    assert score["hard_gate_pass"] is False

def test_aggregates_exclude_missing_usage_instead_of_coercing_to_zero():
    summary = aggregate_cases([{"correct": True, "source_precision": 1.0, "source_recall": 1.0, "unanswerable_correct": None, "permission_leak": False, "mode_match": True, "latency_ms": None, "input_tokens": None, "output_tokens": None}])
    assert summary["latency_p50_ms"] is None
    assert summary["reported_input_tokens"] is None
```

- [ ] **Step 2: Run RED for the new gate cases**

Run the same pytest command as Task 1. Expected: the named gate tests fail because the scorer/aggregator are not implemented.

- [ ] **Step 3: Implement the scorer and aggregation**

Use set intersections over evidence IDs. `source_precision` is intersection/predicted when predictions exist; if no predictions, it is `1.0` only when the case expects none, otherwise `0.0`. `source_recall` is intersection/expected when expected evidence exists; if no expected evidence, it is `1.0`. `correct` compares conclusion-ID sets and requires no permission leak plus exact requested/actual mode match. `unanswerable_correct` is true only for `answerable == false`, status `insufficient_evidence`, and empty evidence/conclusion sets. `permission_leak` is true if returned evidence intersects `forbidden_evidence_ids` or `access_violations` is nonempty. Percentiles use nearest-rank (`rank=max(1, ceil(p*n))` over sorted observed values); no measurements yields null.

- [ ] **Step 4: Run GREEN for scorer and aggregate tests**

Run: `uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q`

Expected: all scorer tests pass without installing new dependencies or contacting services.

### Task 3: Freeze a synthetic Chinese case set and validate its schema

**Files:**
- Create: `semantic/experiments/fixtures/questions.jsonl`
- Modify: `semantic/experiments/test_evaluate.py`

- [ ] **Step 1: Add dataset loading tests before the loader**

Cases must include at least: direct Chinese source lookup; the two-document `A depends_on B` + `B depends_on C` chain; missing premise; conflicting claims; insufficient evidence; deleted/revoked evidence that is forbidden; and a foreign-tenant alias using `e-foreign` as forbidden evidence. Each line contains unique `case_id`, `synthetic: true`, tenant/KB, document revision map, Chinese query, requested mode, expected evidence IDs, expected conclusion IDs, answerable flag, and forbidden evidence IDs. No real tenant/document/user names or contents.

```python
def test_synthetic_question_set_has_unique_cases_and_permission_counterexamples():
    cases = load_cases(FIXTURES / "questions.jsonl")
    assert len({case["case_id"] for case in cases}) == len(cases)
    assert all(case["synthetic"] is True for case in cases)
    assert {case["scenario"] for case in cases} >= {"dependency_chain", "missing_premise", "conflict", "insufficient_evidence", "revoked_source", "foreign_alias"}
    assert any("e-foreign" in case["forbidden_evidence_ids"] for case in cases)
```

- [ ] **Step 2: Run RED for the dataset loader**

Run: `uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q`

Expected: test fails because `questions.jsonl` and/or `load_cases` is absent.

- [ ] **Step 3: Implement strict JSONL loading and synthetic dataset**

Reject malformed JSON, duplicate `case_id`, a missing required field, a case without `synthetic: true`, or a forbidden evidence ID duplicated in expected evidence. Preserve Chinese strings byte-for-byte; do not normalize text or Unicode in the evaluator.

- [ ] **Step 4: Run GREEN for dataset tests**

Run the same test command; expected: all lines load and the foreign/revocation hard-gate fixtures are present.

### Task 4: Add an offline observation runner and safe policy/report artifacts

**Files:**
- Modify: `semantic/experiments/evaluate.py`
- Create: `semantic/experiments/fixtures/evaluation-example-observations.jsonl`
- Create: `docs/plans/semantica/evaluation-baseline.md`
- Create: `docs/plans/semantica/acceptance-policy.json`

- [ ] **Step 1: Write CLI/manifest tests**

The CLI takes exact paths (defined below), joins exact case IDs, rejects duplicate/missing/extra observations, writes per-case rows and an aggregate summary atomically, records evaluator/lock/evidence layer, and never contacts a backend. The example observations are explicitly `evidence_layer: "synthetic"`; they only smoke the parser and scorer. Add the following tests:

```python
def test_evaluate_run_requires_one_observation_for_every_case():
    case = {"case_id": "case-1", "expected_evidence_ids": [], "expected_conclusion_ids": [], "answerable": False, "forbidden_evidence_ids": []}
    with pytest.raises(ValueError):
        evaluate_run([case], [])

def test_example_report_is_synthetic_and_never_approves_policy():
    cases = load_cases(FIXTURES / "questions.jsonl")
    observations = load_observations(FIXTURES / "evaluation-example-observations.jsonl")
    report = evaluate_run(cases, observations)
    policy = json.loads(POLICY.read_text())
    assert report["synthetic_only"] is True
    assert report["evidence_layer"] == "synthetic"
    assert policy["approved"] is False
    assert policy["proposed_thresholds"] is None
```

`evaluation-example-observations.jsonl` contains one row for every question case, all with `evidence_layer: "synthetic"`, `backend: "synthetic-example"`, `latency_ms: null`, `input_tokens: null`, and `output_tokens: null`. These are explicitly illustrative structural outputs, not a query run. Missing measurements must be null, not zero.

- [ ] **Step 2: Run RED, then implement CLI and examples**

Run: `uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q`

Expected RED: CLI or loader assertion fails before the implementation. GREEN: tests pass and generated temporary report explicitly says synthetic-only.

- [ ] **Step 3: Write honest baseline and policy files**

`evaluation-baseline.md` records the synthetic fixture scope, metric definitions, exact commands, and that no native-vs-Semantica query run, controlled-provider, live-model, authorized enterprise corpus, or latency/token measurement is included. `acceptance-policy.json` uses schema version 1, `approved: false`, `proposed_thresholds: null`, and a reason that measured comparable runs and product confirmation are still required. Do not insert invented threshold values or synthetic summary as measured results.

- [ ] **Step 4: Verify the offline tool and classify V03 evidence accurately**

Run: `uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q`

Run: `uv run --locked --project semantic/experiments python semantic/experiments/evaluate.py --dataset semantic/experiments/fixtures/questions.jsonl --observations semantic/experiments/fixtures/evaluation-example-observations.jsonl --output /tmp/semantica-v03-synthetic-report.json`

Expected: exit 0; report labels are synthetic-only, policy approval remains false, and no network/provider/storage call occurs. This verifies the evaluator foundation only; it does **not** verify V03 quality, latency/cost thresholds, controlled-provider, live-model, or promotion.

### Task 5: Commit the offline V03 foundation

**Files:** only those listed in Tasks 1–4 plus `docs/plans/semantica/progress.md`.

- [ ] **Step 1: Record partial status**

Keep V03 `in_progress` with the offline-evaluator evidence and explicit missing actual-runtime comparison. Do not mark V03 verified.

- [ ] **Step 2: Commit scoped files**

Commit as `feat(semantic): v03 offline evaluation foundation`.

## Completion boundary

This plan completes only the evaluator/data-model foundation. V03 remains pending verification until comparable authorized native/Semantica observations, controlled-provider evidence for Chinese Q14/Q17, any available live-model evidence at its own layer, measured latency/token results, and explicit product approval of thresholds are recorded. Until then `policy.approved` remains false and no deployment/promotion claim is allowed.
