# T14 ctnetlink Task 1 Independent Validation

- **Status:** DONE_WITH_CONCERNS
- **Revision:** `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- **Scope:** Independent no-Docker validation of T14 ctnetlink Task 1. No source or test-source files changed. The plan/brief path cited by the implementation report (`docs/plans/2026-09-29-craft-107-t14-ctnetlink-plan.md`) is absent from this checkout; validation scope was reconstructed from `docs/plans/2026-09-29-craft-107-t14-ctnetlink-report.md` and its listed selectors.

## Hash binding

Recomputed source hashes match the Task 1 report's final hashes:

- `deploy/craft/render-boundary/policy-helper/helper.py`: `128a8d6f6173030cb17092ec37fa276fcb0001e0cd21ad0e017db59152f2d6c5`
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`: `ff334bef8d3c54b40ddc006ed9bf9444c80c70d8f9dce05b218ccd469a178090`

The report's stated pre-task hashes cannot be independently rebound to a brief because the brief/plan is missing. HEAD is the revision above; both files are untracked in this worktree.

## Checks run

All commands ran from `deploy/craft/render-boundary/policy-helper` unless noted.

| Command | Result |
|---|---|
| `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` | exit 0; 8 tests passed |
| `python3 -m unittest tests.test_barrier_adapter -v` | exit 0; 18 tests passed |
| `python3 -m py_compile helper.py controller.py barrier_adapter.py` | exit 0 |
| `git diff --check -- deploy/craft/render-boundary/policy-helper/helper.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` (repository root) | exit 0 |

No unittest discovery, Docker commands, image build, or Docker tests were run.

## Acceptance evidence and gaps

The focused parser tests exercise synthetic netlink frames, exact IPv4/IPv6 established tuples, malformed framing/attributes, sender/sequence/error/interruption/limits, and fail-closed behavior. Barrier adapter tests exercise the helper's fixed inventory and owned established-loopback tuple contract. Syntax and whitespace checks pass.

Live ctnetlink behavior against a disposable renderer/helper exact policy flow and disposable-container cleanup were **not validated**. Authentication/authorization, migrations, and application backend API contracts are not applicable to this helper-level network-policy task. The unit fixtures do not establish kernel netlink compatibility, namespace behavior, or cleanup under the live flow. The implementation report also records a previous unsafe unittest-discovery attempt that may have rebuilt a shared helper image tag; this validation did not inspect or alter Docker state.

**Conclusion:** Focused no-Docker checks pass for the report's final source hashes. Task 1's live exact-flow and cleanup acceptance remains open; the missing brief/plan also prevents direct verification of the complete acceptance wording.
