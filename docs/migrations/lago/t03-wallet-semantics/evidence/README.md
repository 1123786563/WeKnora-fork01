# T03 final runtime evidence

Sanitized copies of the four experiment reports from the canonical one-sitting
`run-all` against the isolated pinned stack (Compose project `weknora-lago-75`,
Lago Community `v1.53.0`, digest-locked images, loopback `48893`), plus the
health snapshot taken while the stack was up.

| File | What it proves | How it was produced |
|------|----------------|---------------------|
| `run-all-health.json` | `overall: ready`, release `v1.53.0` from `images.lock.json`, all five pinned digests | `./deploy/lago-lab/wallet-semantics/lab.sh status --json` during the run |
| `e1-expiry.json` | Expiry, no carry-over, termination cadence, wallet cap probe (8 checks) | `LAGO_API_KEY=<operator key> ./deploy/lago-lab/wallet-semantics/lab.sh run e1` |
| `e2-order.json` | Priority/created_at consumption order, granted-before-purchased, monotonic non-negative remainders (5 checks) | `... lab.sh run e2` |
| `e3-concurrency.json` | Duplicate-grant minting + metadata recovery, event idempotency, 6-way concurrent draw totals, void-vs-consume exclusivity, no double-draw across expiry (6 checks) | `... lab.sh run e3` |
| `e4-withdraw.json` | Remainder-only void semantics, refund model, purchased-batch voidability, credit-note premium gate, paid-credit settlement seam (8 checks) | `... lab.sh run e4` |

Contract per report: `run_id`, `status` (`pass | fail | blocked-env`),
per-check `{criterion, expected, observed, outcome}` with outcomes
pre-registered offline (`deploy/lago-lab/wallet-semantics/test_e*_*.py`)
before the run, `created_object_ids`, best-effort `cleanup`, release identity
from the image lock. No secret values; all identities are synthetic
`weknora-t03-<uuid>`.

Note on honest failures: `e1-expiry.json` intentionally contains one failing
check that IS the evidence — `expired_wallet_not_consumable` records the
lazy-termination race window (4000 cents drawn from an expired-but-still-
active wallet, terminated by the hourly clock 1652s after expiry). The cap
probe check passes while recording the blocker data: the seventh concurrent
active wallet is rejected with `wallet_limit_reached` and terminating one
frees the slot. Read both together with `../verdict.md` (invariant a2 is
BLOCKED on that cap).
