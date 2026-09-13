# open-connector contract evidence fixtures (T01)

Runtime evidence for the pinned upstream
`33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a` (`@oomol-lab/open-connector` v1.5.0).
Captured 2026-09-12 by `scripts/open-connector/` probes against a throwaway
local instance of the pinned sources. The gate input is
`contract-report.json`; validate it with:

```sh
python3 scripts/open-connector/contract_gate.py scripts/open-connector/fixtures/contract-report.json
```

## Sanitization scheme

- Every bearer credential is redacted to shape + sha256 prefix:
  `oct_<redacted:xxxxxxxx>` for stored runtime tokens,
  `oc-t01-admin-<redacted:xxxxxxxx>` for the locally minted admin token,
  `oc-t01-local-enc-<redacted:xxxxxxxx>` for the locally minted encryption key.
  No raw token material appears in any fixture.
- All provider API keys are locally fabricated values named `oc-t01-fake-*`;
  no real provider account was used. dune/postmark/hasdata were chosen because
  they declare no `credentialValidators`, so fake keys store without provider
  verification.
- OAuth client `oc-t01-fake-client-id` / `oc-t01-fake-client-secret` are
  fabricated; the GitHub token exchange provably rejects them (that rejection
  is itself captured evidence in `oauth_correlation.json`).
- Random Idempotency-Keys are shown as `oc-t01-…-<random>`.

## Environment provenance

- Upstream clone: `.oc-upstream-33dd4ad` (isolated, gitignored, detached at
  the pinned SHA).
- Probe server: `node src/server/index.ts` at `127.0.0.1:31701`, sqlite under
  `/tmp/oc-t01-data-a` (throwaway, removed after probes),
  `OOMOL_CONNECT_ADMIN_TOKEN` set, egress trust for `export.arxiv.org` /
  `api.crossref.org` (host DNS resolves some public APIs to private space),
  custom OAuth allowed for `github` with a locally minted encryption key.
- The no-auth control probe ran a second throwaway instance at
  `127.0.0.1:31702` (`/tmp/oc-t01-data-b`) with no auth env at all.
- Clock-dependent cases (`expired_key`) and the injected-failure case
  (`audit_failure`) use upstream's own vitest suite at the pinned SHA —
  method noted inside each fixture.

## Case index

| fixture | live method |
| --- | --- |
| cross_connection.json | HTTP 403 connection_not_allowed (pre-credential) |
| empty_grant.json | HTTP authorization_failed (grant layer allowed; provider rejected fake key) |
| default_alias.json | 200 via arxiv:default; 403/404 no-fallback on accuweather |
| no_auth.json | 200 through virtual arxiv:default with an unrelated token grant |
| admin_denied.json | 401 on /api/* with runtime token; open-hazard control 200 |
| proxy_denied.json | HTTP 403 proxy_not_allowed (empty allowedProxies) |
| oauth_correlation.json | initiation/poll/callback-rejection state machine |
| key_replay.json | same meta.executionId on repeated identical request |
| key_conflict.json | HTTP 409 idempotency_key_conflict |
| in_progress.json | live parallel race + upstream deterministic test |
| expired_key.json | upstream controllable-clock unit test (24h) |
| audit_failure.json | upstream injected-failure unit test (runs.add throws) |

## T18 real-provider read-only fixture

`provider_read_runtime.json` (added 2026-09-13, coordinator ruling R21) records the
T18 READ-ONLY probe against **real GitHub** on the T16 release-candidate image
digest `sha256:4de6df4d…` (not the T01 host-node instance): restricted oct_
token with non-empty grants executed `github.get_current_user` (HTTP 200), plus
pre-execution negative controls (400 `action_not_allowed`, 403
`connection_not_allowed`) and the zero-residue cleanup record. Unlike the T01
cases above, a REAL provider credential (host gh CLI keyring) was used —
strictly read-only per the session's authorization boundary; the credential is
redacted everywhere and was never echoed. Raw probe response files were
destroyed with the throwaway container (their sha256 fingerprints are recorded
inside the fixture and in `docs/integrations/open-connector-release.md` §6).
