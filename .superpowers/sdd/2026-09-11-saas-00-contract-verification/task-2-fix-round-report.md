# V02 fix round

Addressed review findings: V01 JSON inventory is consumed directly (while
OpenAPI YAML remains supported), HTTP errors return contract failure 1,
cleanup is capture and namespace guarded with unsettled transaction refusal,
write replays can send an idempotency key and must return the same result,
failure paths persist redacted `run.json`, and embedded namespace or tenant
values are checked.

Focused tests: `python3 -m unittest scripts.saas.probe_case_test -v` — passed;
`python3 -m py_compile scripts/saas/probe_case.py` — passed;
`git diff --check` — passed. Live experiments remain blocked-env.
