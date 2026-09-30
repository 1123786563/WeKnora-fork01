# T14 immutable renderer startup failure — durable evidence addendum

Date: 2026-09-28 Asia/Shanghai. This records the raw evidence from the already-authorized bounded live probe; it does not rerun Docker or claim T14 acceptance.

- Source run/report: `/tmp/craft107-t14-live.aN19RZ`, summarized in `docs/plans/2026-09-28-craft-107-t14-live-probe-report.md`.
- Candidate image: `sha256:a2dc460f05d310ad0de3f82658a83676d71db6628832ad0d9958075d3893b590`, untagged, `linux/arm64`.
- Runtime observed: Python 3.11 venv, Chromium 153.0.8010.52, ChromeDriver 153.0.8010.52, Selenium 4.35.0.
- Disposition: Selenium fails while constructing the first Chrome session with `SessionNotCreatedException: Chrome instance exited`; no attempt matrix, policy windows, or acceptance evidence was produced. Host barrier correctly marked the run incomplete. Renderer cleanup is verified; `network_mode=none`, no mounts/ports, non-root user, read-only root, `cap_drop=ALL`, and bounded `/tmp`/`/out` tmpfs are recorded in the inspect snapshot.
- Durable raw evidence and integrity manifest: `docs/testing/craft/t14/2026-09-28-live-chrome-startup-failure/`. All manifest checksums verified after copy from the exact `/tmp` run directory.

The error text does not identify why Chrome exited. The Chromium/ChromeDriver startup failure is a verified runtime blocker; it must be diagnosed and the immutable live probe rerun before T14 is verified. This diagnostic is separate from the earlier WebDriver loopback-flow root cause and does not establish whether that policy repair works.
