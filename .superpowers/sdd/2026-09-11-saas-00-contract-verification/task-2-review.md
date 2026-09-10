# V02 review: FAIL

Reviewed commit: `a17bf4d4ce95e3ae6f4ccdd1db98d790be472b64`
Scope: `scripts/saas/probe_case.py`, `scripts/saas/probe_case_test.py`,
`deploy/openmeter/contract-cases/README.md`, and the V02 progress entry.

## Evidence

- `python3 -m unittest scripts.saas.probe_case_test -v`: **PASS**, 5/5.
- `python3 -m py_compile scripts/saas/probe_case.py`: **PASS**.
- `git diff --check`: **PASS** for the reviewed commit.
- The implementation was read against Task 2 in
  `docs/superpowers/plans/2026-09-11-saas-00-contract-verification.md`.
- No real OpenMeter business experiment was run; the progress ledger correctly
  leaves OM-02--OM-10 as blocked-environment work.

## Blocking findings

1. **Default schema input is unusable (functional, high).**

   `main()` defaults `--schema` to
   `docs/superpowers/specs/evidence/2026-09-10-saas-billing-interface-inventory.json`,
   which is V01’s JSON inventory. `run_case()` passes that JSON text to
   `operation_paths()`, which only accepts the fixed OpenAPI YAML `paths:` shape.
   Therefore the documented command without an explicit `--schema` fails before
   any case can run. The default must resolve to the fixed official OpenAPI YAML
   (or the runner must explicitly consume the V01 inventory’s `operations`
   mapping); add a regression test for the documented command/schema path.

2. **Non-2xx responses are reported with the wrong exit class (functional,
   high).**

   `urlopen()` raises `HTTPError` for HTTP 4xx/5xx before returning a status.
   Since `HTTPError` is an `URLError` subclass and the `URLError` handler comes
   first, `main()` returns 2 (`blocked-env`) instead of the required 1 for a
   non-2xx response. Handle `HTTPError` before `URLError` (and preserve its
   response/status in the per-step evidence), with a regression test.

3. **Cleanup contract is declared but never executed (functional, high).**

   `case["cleanup"]` is never validated or run. A successful write case can
   leave test Customers or other objects behind, violating the requirement that
   cleanup only handles objects created in the explicit namespace and only when
   there are no unsettled transactions. Implement a constrained cleanup phase
   driven by captured IDs, enforce the namespace and unsettled-transaction guard,
   and record cleanup outcomes; add tests for cross-namespace and unsettled
   refusal.

4. **Idempotency is absent (functional, high).**

   The task requires reproducible commercial contract experiments, while the
   runner sends no idempotency key, records no idempotency query rule, and has no
   replay/assertion handling. Add case-level/step-level idempotency configuration,
   send the documented key on writes, replay the request where required, and
   assert the same business result/object rather than creating a duplicate.
   Include a test proving a replay cannot silently create a second resource.

5. **Failed steps do not persist the required per-step artifacts (medium).**

   `run_case()` writes `run.json` only after all steps and final assertions
   succeed. A failed HTTP request, business assertion, capture, or cleanup
   therefore loses the request/response evidence needed to diagnose the case.
   Persist redacted records in a `finally`/failure path while retaining the
   non-zero exit code, and test artifact creation on an assertion failure.

## Security/negative-path assessment

The reviewed tests do cover recursive nested business assertions, default write
refusal, explicit namespace mismatch, unknown captures, JSON Pointer decoding,
and the README documents redaction and write gating. `_bind()` also rejects an
undeclared capture at execution time. However, namespace checking only examines
the optional `step["namespace"]`; arbitrary namespace fields embedded in a
write path/body are not constrained by validation. The fix should enforce that
all namespace-bearing request values equal the explicit namespace (or reject
unsupported namespace fields), and should add a negative test. Redaction is
case-insensitive for the listed secret key names, but artifact persistence must
also cover failures as described above.

## Decision

**FAIL.** The unit tests are green, but the documented runner cannot run with
its default schema and several required safety/reproducibility behaviors are
missing. Exact remediation is listed above; re-review after those changes and
new focused tests are added. Do not mark V02 `done` or allow V03 to depend on
this runner until the blocking findings are resolved.

## Round-1 re-review: FAIL

Reviewed fix commit `69e5248ce229717ab58ed9c404f29eec8e4df982`.

Evidence: `python3 -m unittest scripts.saas.probe_case_test -v` passes 5/5;
the commit passes `git diff --check`. The JSON inventory fallback, exception
ordering, success/failure artifact `finally`, cleanup hook, and optional replay
hook address parts of the first review.

Remaining blocking findings:

1. Cleanup sends `DELETE` regardless of `allow_test_writes`. The documented
   default write refusal can therefore be bypassed using a cleanup-only case.
   Require the explicit write flag and test it.

2. `_walk()` yields dictionary keys as well as values. A valid body such as
   `{"namespace": "saas-x"}` is rejected because the key `namespace` is
   compared with the namespace. Capture references are also exempted without
   checking their resolved namespace. Inspect key/value pairs deliberately,
   bind captures first, and add same-namespace and cross-namespace tests.

3. Cleanup directly indexes `captures[item["capture"]]`, uses weak string
   replacement, and does not validate an absolute path or embedded namespace.
   Reuse normal JSON Pointer/capture binding and return clean validation errors
   for unknown captures; enforce the unsettled-transaction guard at deletion.

4. Idempotency remains optional: writes do not require an idempotency key and
   replay is only performed when `replay` is set. Require metadata for
   repeatable writes and assert resource/business identity on replay, with
   missing-key and duplicate-creation tests.

5. HTTP errors are still absent from per-step artifacts. `urlopen()` raises
   `HTTPError` before `_request_json()` returns, so `run.json` has no failed
   request/response record. Record the error inside the step path before
   re-raising, and test failure artifact contents.

The re-review remains **FAIL** until these authorization, namespace, cleanup,
idempotency, and failure-evidence issues are fixed.

## Round-2 re-review: FAIL

Reviewed fix commit `88f4129821001be14f316692bf319c604edffa3b`.

Evidence: `python3 -m unittest scripts.saas.probe_case_test -v` passes 7/7 and
`git diff 88f4129^ 88f4129 --check` passes. The fix adds cleanup write gating,
same-namespace body support, repeatable-write key validation, bound cleanup
substitution, and in-step HTTP error recording.

Remaining blockers:

1. Cleanup checks only `item["namespace"]` and a leading slash. It does not
   inspect namespace or tenant values embedded in the cleanup path, so a path
   such as `/namespaces/other/customers/${capture:id}` can issue a
   cross-namespace DELETE. Apply the same bound namespace-pair validation to
   cleanup paths and add a regression test.

2. Cleanup accepts a declared capture that no preceding step actually produced;
   execution then leaks `KeyError` at `captures[item["capture"]]`. Require a
   proven preceding capture or raise a clean `ValueError`, with a test.

3. Replay compares complete responses only when `step.replay` is enabled and
   does not require a configured resource identity assertion. A case omitting
   the identity can therefore fail to prove same-resource replay. Require a
   replay identity JSON Pointer/capture assertion and test differing resource
   IDs.

V02 remains **FAIL** until these cleanup and replay contract gaps are fixed.

## Round-3 re-review: FAIL

Reviewed fix commit `8c9b477742ab357ccd8f8a27cda293d12f0d521d`.

Evidence: `python3 -m unittest scripts.saas.probe_case_test -v` passes 8/8;
`git diff 8c9b477^ 8c9b477 --check` passes. The commit adds unavailable
cleanup-capture rejection and requires a replay identity pointer, then compares
that pointer across the original and replay responses.

Remaining blocker:

1. **Cleanup path namespace enforcement still does not work for paths.**
   `_namespace_pairs()` only traverses dictionaries and lists. Cleanup paths are
   strings (for example `/namespaces/other/customers/${capture:id}`), so the
   new loop yields no pairs and the cross-namespace path proceeds to DELETE.
   The same issue exists for request `step["path"]`. Parse/validate path
   namespace segments explicitly (or reject path forms whose namespace cannot be
   proven equal to the explicit namespace), after capture binding, and add the
   promised cross-namespace cleanup-path regression test. Also validate that
   `replay_identity` is an absolute JSON Pointer during case validation rather
   than deferring malformed configuration to a live run.

V02 remains **FAIL**. The focused suite is green, but a cross-namespace cleanup
DELETE is still possible under the current implementation.
