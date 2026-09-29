# Task Brief — DOC2: Correct current iOS device acceptance status

## Authority and finding

- Parent plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-final-review-repairs.md`, task DOC2.
- Independent finding: `.superpowers/sdd/plan-t31-ios27/final-review.md`, T31-F3 Low.
- Exact initial HEAD: `a2c695f18`; execute in dedicated acceptance-record worktree after fast-forwarding it to include this brief commit.
- Current document says “native build passed; JS startup unresolved” while later sections and tracked R4 artifacts record a successful Release launch and safe-area screen.

## Worktree, ownership, and interfaces

- Execute in `/Users/wuyongjun/.paseo/worktrees/144ixsa6/t31-doc2-acceptance-record`, branch `codex/t31-doc2-acceptance-record`.
- Role: mechanical_worker. No subagents.
- Own only `docs/testing/mobile-runtime-login-device-acceptance.md` and `.superpowers/sdd/plan-t31-ios27/doc2-report.md`.
- Use tracked R4 evidence: `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/ios27-release-launch-r1.png`, `simctl-launch-r1.txt`, `simctl-install-launch.txt`, and `xcodebuild-release-final.log.gz`; hashes/details are also in `.superpowers/sdd/plan-t31-ios27/fix-task-4-report.md`.

## Acceptance

1. Update current iOS status to reflect successful iOS 27 simulator Release launch and visible login safe-area layout; cite the fresh R4 screenshot and launch transcript.
2. State accurately that the bounded `simctl launch --console` output reached JavaScript bundle evaluation and was terminated by the 15-second timeout (exit 124); do not claim it proves long-duration stability or interaction/authentication.
3. Keep staging password/OIDC login, real Deployment capability negotiation, and Android physical-device acceptance explicitly pending; retain useful historical failure notes without presenting them as the current iOS state.
4. Confirm every evidence path exists. Do not edit evidence, redact/alter logs, or change source.
5. Run `git diff --check`, record exact commands/results in `doc2-report.md`, and commit only owned doc/report files locally.
6. Stop for independent documentation review; do not self-approve.
