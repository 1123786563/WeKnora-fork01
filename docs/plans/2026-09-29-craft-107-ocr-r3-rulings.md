
## Post-audit repair results and residual low findings

- Budget shape Medium: fixed and integrated. Malformed optional `extension_action` now degrades to `null` while required pause fields remain strict; no action control renders without a validated server tuple. Independent review PASS/PASS; API client 15/15, Craft route 13/13, shared Craft suite 187/187 on integration, web typecheck and `git diff --check` passed. See the R3 budget report/review.
- Draft-head manifest Medium: fixed and integrated. Transaction checks the version digest against the selected head and the identical reversed-order retry now succeeds after canonical path sorting. Independent review PASS/PASS; focused repository selector passed in integration. See version manifest Fix1 report/review/checkpoint.
- Java launcher High/Medium: class-path forms/wrappers and `-ea:` were fixed in Fix1; module path, upgrade module path and patch module forms fixed in Fix2. Fix2 independent review found a separate HIGH `@argfile` bypass: Java expands `@file` before the screened options. Fix3 is in progress; no final Java policy acceptance until it passes review.
- F08 production build receipt caller: remains valid and blocked. Repository persistence exists, but no production dispatcher supplies service-observed terminal exit code through T04 gate + T03 policy + T19 normal executor. Writable build logs remain non-authoritative and successful production builds remain `not_run`. No safe inference from tests or migrations closes this behavior.

The following R3 LOW findings were independently confirmed by code inspection and are deferred with explicit rationale; they are not acceptance evidence and do not close adjacent Medium findings:

| Finding | Ruling | Reason / status |
| --- | --- | --- |
| `CraftWebBuildReview` constructor does not require `RuntimeDigest` | Defer low | Current sole production assembly supplies a non-empty digest; a second constructor can silently weaken anchoring. No immediate production path presently omits it. Fix is small but coupled to T04 build gate assembly; include in the next scoped T04 repair if that seam is edited. |
| `OutputGeneration == ""` nested receipt check is unreachable after earlier required-text validation | Defer low | Redundant defensive condition; it does not change accepted inputs. F08 receipt caller remains blocked independently. |
| Promotion cursor seed missing is wrapped as `ErrInvalidInput` | Defer low | Current caller logs and does not branch on sentinel, so no observed behavior changes. Correct typing is useful only when adding retry/error classification. |
| Promotion claim `RowsAffected != 1` has no typed sentinel | Defer low | Current callers only log the error; not currently used for retry classification. Revisit when claim callers need a recovery distinction. |
| Zero-consumer `canExtend`, public `CraftBudgetPause` export, non-null assertion, empty fragment | Fixed | Removed obsolete `canExtend`, exported required type, and simplified route guard/markup within reviewed budget fix. |

OCR R3 remains partial coverage: HTTP 429 left 7/27 selected items without review. These rulings are local evidence only; complete workspace OCR is still required after provider reset.

### Java module/argfile authority

The Java path-list finding and the argfile finding are distinct. Fix2 closes explicit `--module-path`/`-p`, `--upgrade-module-path`, and `--patch-module` tokens, while the later Fix3 treats argfiles as a separate expansion channel. Oracle JDK 21 launcher documentation states that `@` file contents are expanded before option processing, supports recursive/extra `@` indirection only within documented limits, and permits `JDK_JAVA_OPTIONS` to prepend launcher options (including argfiles). The policy layer cannot inspect arbitrary file bytes or a server-injected environment after dispatch. Fix3 therefore needs to deny Java `@` argfile usage and any `JDK_JAVA_OPTIONS` injection that the request seam accepts, while preserving ordinary explicit launcher flags. Primary reference: [Oracle JDK 21 `java` command documentation](https://docs.oracle.com/en/java/javase/21/docs/specs/man/java.html).
