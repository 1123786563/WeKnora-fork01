# T14 WebDriver Control Policy — Independent Review

Date: 2026-09-29 (Asia/Shanghai)
Reviewed package: `.superpowers/sdd/2026-09-28-craft-107-t14-webdriver-control-policy-plan/review-package-01.tar.gz`
Package SHA-256: `fa25d7e73e9171e9fc8434a1f9b9e9379a2d50e4104a8e6a404680203de08819`
Review agent: `/root/t14_policy_security_review` (read-only)
Validator: `/root/t14_policy_behavior_validation` (read-only)

## Verdict

- **Spec compliance: FAIL.** T14 live acceptance remains unverified; Docker-backed namespace integration timed out before policy install, conntrack proof, or canary.
- **Code quality: FAIL.** Findings below must be fixed and independently re-reviewed before integration.
- Manifest hashes passed verification. Original untracked source bodies were not captured at task start, so no trustworthy text delta exists; review inspected the full final files.
- Scoped non-Docker validation: 24 tests passed, 1 Docker-gated test skipped; `py_compile` and `git diff --check` passed. These do not prove live namespace behavior.

## Findings

### High — socket discovery filters use states that were already normalized

`controller.py` converts `/proc` TCP states `01` and `0A` to `ESTABLISHED` and `LISTEN`, but then filters against the raw codes. Both discovered socket arrays remain empty, making policy installation impossible. Fix the filters and add regression coverage that executes the emitted discovery script against a real socket fixture or equivalent bounded proc fixture.

### High — conntrack acceptance is not an exact complete-flow proof

`helper.py` accepts a line after matching the first src/dst/ports and the word `ESTABLISHED`; it does not verify TCP, family, reply tuple, or complete-record bounds. Read a bounded complete table, reject overflow/truncation, and match exactly one IPv4/IPv6 TCP ESTABLISHED record with both original and reply tuple directions.

### Medium — canary can pass without a connection attempt

`controller.py` treats every nonzero helper exit as a denied connection. A script error or source-port assertion before `connect()` can combine with unrelated counter activity and pass. The receipt records only an “ephemeral-distinct” claim instead of actual source port/attempt outcome. Require structured evidence that connect was attempted to the attested listener using a distinct source port and failed for an expected policy-drop outcome, with a positive counter delta in a narrow before/after window.

### Medium — discovery output and proc inventory are unbounded

The eight-second subprocess timeout bounds elapsed time but `capture_output` does not bound bytes or memory. The inline script enumerates all proc TCP rows and socket FDs. Bound per-table rows/FDs and serialized output, cap controller-side stdout/stderr bytes, and reject overflow.

### High — integration fixture does not exercise post-policy reuse or fresh denial separately

The Docker fixture leaves its established client socket open but sends no application data after policy installation. The assertion checks only an aggregate canary result; it does not prove that the existing tuple still carries traffic or that a new same-listener connection is denied and counted. Add separate assertions and run the disposable namespace test successfully before any immutable T14 probe.

## Preserved properties confirmed by review

- Output chain priority `-150` is after conntrack priority `-200`.
- The generated exceptions are two full-tuple bidirectional rules with `ct state established`.
- Preview-only allow rules, loopback drops, target counters, and the T14 attempt inventory remain present.

## Validation report

`validator-report-task-1.md` records the exact 24-pass/1-skip non-live result and confirms no Docker or immutable candidate probe was run.
