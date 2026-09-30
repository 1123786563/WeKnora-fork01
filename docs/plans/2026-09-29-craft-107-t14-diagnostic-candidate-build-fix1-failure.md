# T14 Candidate Build Attempt — Fix1 Trace Source

## Result

The reviewed candidate builder did not produce a candidate. It passed exact source ID and all six overlay hash gates, exported the never-started source container, then remained in `docker import` until the outer 900-second bound ended. The builder cleanup trap removed the temporary container and export directory. No browser, policy, network, release receipt, or matrix attempt ran.

## Inputs and evidence

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`, source image `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27`.
- Final diagnostic `probe.py` SHA-256: `1f0d7a0a1fd871356061a9bb5f5250965898ebc1f765e38b4bbba7f090cee81e`; builder and topology-test pins matched. Other five overlay hashes stayed at their reviewed values.
- Command: `timeout 900 deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27`.
- Full captured stdout/stderr: `/tmp/craft107-t14-candidate-build-fix1.log`, SHA-256 `47f590d77141d6104227996159f0f0cf099bc2caa848837f67230a6b19d99791`.
- Builder printed source ID, overlay container ID `fb37268360095203cae5da9493ea2d324e89ee20a4900c9f8926c5e575911c75`, and each exact overlay destination. It then completed `docker export` with archive SHA-256 `fcfcef86ec51816ef455eca93da80ddca4833dd78104c7cdbfa181a7e3c12059`.
- The only subsequent child command observed was `docker import --platform linux/arm64 ... rootfs.tar`. It remained active for approximately 9 minutes after export and was still sleeping on the Docker CLI when the parent 900-second timeout expired. The candidate ID/verification log lines never appeared.
- The wrapper used zsh variable name `status` to capture the command exit; zsh treats that variable as read-only, so it did not print the outer `timeout` status. Do not report an observed exit code. The 900-second parent process ended, the builder printed `cleanup_container_id=... exit=0` and `cleanup_temp_exit=0`, and a subsequent inspect returned `No such container`.
- `docker image inspect sha256:c115e66adc210e483f72525f62c544f17d8b5e9808acb1ad3e21da9b85650894` confirms the earlier pre-instrumentation candidate exists; it is not usable for this final source hash and was not run.

## Disposition

Candidate construction is blocked at local Docker import performance for this 1.6 GiB exported rootfs. Do not reuse the old candidate or relax exact-source checks. An independent architecture audit is assessing whether the previously verified candidate can be safely used as a base to replace only the fixed-hash probe file while preserving exact source provenance and volume-free topology. No further build retry is authorized until that audit identifies a bounded, reviewable method or the Docker resource condition changes.
