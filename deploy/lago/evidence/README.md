# Lago T01 runtime evidence

Sanitized, reviewable evidence that the pinned Lago Community `v1.53.0`
environment really came up, passed its health rules, executed the real
Customer create/delete contract against the pinned API, and shut down
without data loss. Captured 2026-09-20 on the T01 worktree
(`lago-73-community-env`).

## Evidence contract

- Every file here must be **verbatim tool output or a sanitized operator
  narrative**; no hand-edited verdicts, no re-typed numbers.
- **No secret values.** No `.env` content, no API key material, no JWTs,
  no passwords. Synthetic probe customer ids and public image digests are
  fine. Before committing anything here, grep the tracked tree for the
  live secret values — zero hits is the bar.
- A blocked environment is recorded as a `blocked-env` probe report (or
  `unavailable` health snapshot) and clearly labeled as such; it is never
  presented as a passed contract.
- These files evidence the **Community** runtime only. They are not AGPL
  legal approval and not Premium capability evidence.

## Files

| File | What it proves | How it was produced |
|------|----------------|---------------------|
| `t01-run.txt` | Timeline of the full real run: pull, readiness, seeding, probe, `down`, volume survival | sanitized operator narrative with UTC timestamps |
| `t01-health.json` | `overall: ready` — API `/health` ok, api/api-worker/db/redis Compose-healthy, api-clock running, release v1.53.0 + all five digests from the lock | `./deploy/lago/lago.sh status --json` (verbatim stdout) |
| `t01-contract.json` | Real contract pass on the pinned runtime: `GET /health` 200, `POST /api/v1/customers` 200 (created `weknora-t01-probe-<uuid>`), `DELETE /api/v1/customers/<id>` 200 (deleted); status `pass` | `LAGO_API_KEY=<operator key> ./deploy/lago/lago.sh contract-probe --output ...` (verbatim sanitized report) |
| `t01-images.txt` | The five digests actually pulled and run locally equal `images.lock.json` | `docker image inspect` per locked reference, generated after `down` |

## Regenerating

Re-run the workflow from [`../README.md`](../README.md) and overwrite the
files with fresh output. The probe customer id and timestamps will differ;
`overall: ready` and `status: pass` are the expected stable outcomes on
the pinned release.
