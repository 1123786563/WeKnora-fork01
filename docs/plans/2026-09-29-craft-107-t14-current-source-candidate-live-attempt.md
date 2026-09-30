# T14 Current-source candidate live attempt 1

Date: 2026-09-29 (UTC)

## Candidate build

- Source renderer image: `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27` (`linux/arm64`).
- Builder: `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`; builder source hash `aca065850a0e49ab47e0675567066d5c9ee424a37438acbfd835ed5ed89403c0`; test hash `ff27bdb11593a95096e6ec93e43afec70d4d483fa54bfc147ff3acd94521cdc3`.
- Focused builder topology checks: `python3 -m unittest deploy.craft.render-boundary.test_build_volume_free_diagnostics_candidate -v`, 4/4 pass. `sh -n deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` pass.
- Exact command: `timeout 900 deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27`.
- Export tar SHA-256: `5ed270f88050d7f38fec4601912ba3a67f034dc9eefb5108f469bb74644d6a67`.
- Candidate ID: `sha256:c115e66adc210e483f72525f62c544f17d8b5e9808acb1ad3e21da9b85650894`.
- Candidate inspect and helper checks confirmed linux/arm64, no repo tags, no image volume declarations, runtime config equal to the exact source except `Volumes` and `ArgsEscaped`; the six embedded files matched current source hashes including probe `6537b05abdcdd917a95b5ae65248b66e87ed717aa61ccab2788f805a6376157b` and preview app `83173b30c49d075d8d9d89a04fca66cc7bf9d45de077b28b2cc45f06166738ea`.
- Build verification containers were never started and both were removed (exit 0); temporary export directory removed (exit 0). Candidate retained by immutable ID only.

## Live attempt

- Exact command: `CRAFT_RENDER_BOUNDARY_IMAGE=sha256:c115e66adc210e483f72525f62c544f17d8b5e9808acb1ad3e21da9b85650894 CRAFT_T14_POLICY_HELPER_IMAGE=sha256:c0cfa88b4db1940b7cbc978f0b43e713c22ce4af0ebcdeb2ec7db9a14bf1f2bb timeout --signal=TERM --kill-after=15s 1200 ./deploy/craft/render-boundary/run-probe.sh docs/testing/craft/t14/2026-09-29-browser-matrix`.
- Exit 1. Runner extracted candidate `/opt/probe.py` and all five preview files and passed its exact source manifest gate. Runtime was Chromium 153.0.8010.52 / ChromeDriver 153.0.8010.52 / Selenium 4.35.0. Renderer namespace and security/resource configuration were captured.
- Host barrier timed out after its 30-second bound waiting for `001-diagnostic_complete-receipt.json` (sequence 1, before policy installation, release acknowledgement, and before any browser attempt). `completed_windows=[]`, `controller_records=[]`, no browser attempts receive credit. Renderer stdout/stderr had only version preflight and empty stderr; stage-start records for the post-release sequence were not reached.
- The final host `docker exec` bounded read itself timed out at 1 second while the receipt was absent; this is a secondary host-side symptom after the 30-second missing-receipt wait, not evidence the receipt existed.
- Cleanup: renderer absent confirmed, removal exit 0, no helper IDs to remove, no cleanup errors; `cleanup.json`=`verified-clean`.

## Disposition

The exact-source image production blocker is resolved for this checkpoint. T14 remains unverified: this run did not reach matrix attempts. Root-cause investigation is active against renderer startup/initial UX/control flow and host receipt transport, using this immutable candidate and retained evidence. Do not extend barrier timeouts, weaken receipt requirements, or credit any of the 37 attempts based on this run.

Raw evidence: `docs/testing/craft/t14/2026-09-29-browser-matrix/`; SHA-256 manifest in that directory covers retained files. Next attempt only after root cause and an independently reviewed narrow repair or a reproducible environmental explanation is established.
