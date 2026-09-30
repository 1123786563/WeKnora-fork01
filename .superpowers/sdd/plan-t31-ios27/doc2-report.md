# DOC2 implementation report — iOS device acceptance status

## Change

Updated `docs/testing/mobile-runtime-login-device-acceptance.md` to record the
successful iOS 27 simulator Release launch and visible login safe-area layout
from tracked R4 evidence. The document explicitly records that the bounded
`simctl launch --console` reached JavaScript bundle evaluation and ended at its
15-second limit with exit 124; it makes no claim about long-duration stability,
interaction, or authentication. Staging password/OIDC login, real Deployment
capability negotiation, and Android physical-device acceptance remain Pending.
Historical Debug failures are retained as diagnostics.

## Evidence path verification

Command:

```sh
for f in \
  docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/ios27-release-launch-r1.png \
  docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/simctl-launch-r1.txt \
  docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/simctl-install-launch.txt \
  docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/xcodebuild-release-final.log.gz
do test -f "$f" && printf 'EXISTS %s\n' "$f" || printf 'MISSING %s\n' "$f"; done
```

Result: all four paths reported `EXISTS`. The launch transcript contains
`ReactInstance: evaluateJavaScript() with JS bundle` and
`LAUNCH_COMMAND_EXIT=124`. `gzip -t
docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/xcodebuild-release-final.log.gz`
completed successfully. No evidence files were edited.

## Diff validation and commit

Command: `git diff --check` — exit 0, no output.

Command: `git diff -- docs/testing/mobile-runtime-login-device-acceptance.md .superpowers/sdd/plan-t31-ios27/doc2-report.md`
— reviewed the owned documentation changes.

Command:

```sh
git add docs/testing/mobile-runtime-login-device-acceptance.md \
  .superpowers/sdd/plan-t31-ios27/doc2-report.md
git commit -m "docs: correct iOS simulator acceptance status"
```

Commit is limited to those two owned files. Independent documentation review
is pending; this report does not self-approve the change.
