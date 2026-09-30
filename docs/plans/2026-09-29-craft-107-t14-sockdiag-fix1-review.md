# Independent review — T14 SOCK_DIAG Fix 1

Reviewed 2026-09-29, read only except for this report. Scope is solely the IPv6 address canonicalization repair for finding 2 in `2026-09-29-craft-107-t14-sockdiag-ack-policy-review.md`. Basis: approved Craft Spec stories 12/14 and no-egress invariant, `CONTEXT.md`, ADR 0004, SOCK_DIAG architecture ruling, Fix 1 plan and brief, prior review, and the exact two-file task delta. No OCR or Docker was run.

| File | Pinned baseline SHA-256 | Reviewed SHA-256 |
| --- | --- | --- |
| `deploy/craft/render-boundary/policy-helper/helper.py` | `c3d40a6a1d5ba9ef21c15cbabe34d2964a282544586df83a3a1f0e224c248a6a` | `c8d30121927350e3cea5fd3b5823ca2308f1b4f550af1d730d29ac7e92fd87c2` |
| `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` | `5756944010c01f8bb0a6e2d701e72c938425db7ed63c9ae3268d8eb0c0584db6` | `4026dc762d2e5f7c5535c4c0874279300fb110d9a06042739ab3f77fd1333167` |

The files are untracked, so Git has no task baseline diff for them. I independently reversed only the reported changes in memory and obtained both pinned baseline hashes exactly. The scoped delta is the canonical expected source/destination addresses in `parse_sock_diag_dump` (`helper.py:208-212`) and the expanded IPv6 test (`test_controller_integration.py:146-158`). HEAD was `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`.

## Findings

No defect blocks acceptance of this canonicalization repair. The prior review's live ACK-counter and same-source-port acceptance findings remain separate parent work; this repair neither claims nor supplies those receipts.

## Verdicts

**Spec compliance: PASS for Fix 1.** `webdriver_rules` validates the unchanged seven-field contract, matching family, loopback addresses, ports, state, and inode before `parse_sock_diag_dump` normalizes the two expected address strings (`helper.py:111-131,205-213`). The kernel record uses the same `ipaddress.ip_address(...).compressed` representation (`helper.py:194-198`). The subsequent equality still requires the full tuple, TCP_ESTABLISHED state, and exact inode, with exactly one match (`helper.py:278-283`). No public flow field, nft rule, or ACK policy changed.

**Code quality: PASS for Fix 1.** The new test supplies expanded `0:0:0:0:0:0:0:1` input while constructing the diagnostic record with `::1`, then asserts the complete canonical tuple. This directly exercises the previous false absence. Re-parsing validated addresses is redundant but deterministic and harmless; it creates no broader accepted address family or loopback set. Existing rejection tests remain in place.

## Verification

- `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest` with the exact IPv4/IPv6, expanded IPv6, and bad/incomplete SOCK_DIAG parser selectors: **3 passed**.
- `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest tests.test_barrier_adapter -q`: **18 passed**.
- `git diff --check`: **exit 0**.

These checks verify the scoped repair at the pinned hashes. The parent still needs the separate live acceptance evidence required by the original T14 plan before accepting all of T14.
