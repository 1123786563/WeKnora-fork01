# T14 browser runtime feasibility probe

Date: 2026-09-24. Scope: read-only local runtime investigation for the T14 render-boundary proof. No production, runner, test, issue, image, or remote state was changed. No image pull or build was run.

## Result

**Verified blocker:** this host has no locally usable Linux/arm64 Chromium runtime matching the existing runner. The Docker engine is Linux/arm64, but the local image inventory contains no Chromium or ChromeDriver; the host browser is a macOS application and cannot satisfy the required isolated Linux renderer.

The concrete reproducible next route is the digest-pinned official Selenium image:

```
ghcr.io/seleniumhq/standalone-chromium:nightly@sha256:d9a71401975097d3c7ea125aa7e626b334dac2d96d082f47eebf7ca928db9618
```

When present locally, set `CRAFT_RENDER_BOUNDARY_IMAGE` to that immutable reference and run the existing `deploy/craft/render-boundary/run-probe.sh`. The runner verifies exact RepoDigest and `linux/arm64`, stages the probe into a stopped container, and preserves `network none`, read-only root, non-root UID, dropped capabilities, resource limits, and no mount/port constraints. This is a recommendation for a resource-capable environment, not a local success claim.

## Verified facts

Commands run from the integration worktree:

```
uname -a
Darwin ... RELEASE_ARM64_T8142 ...
uname -m
arm64
docker version
client=29.4.0 server=29.4.0 arch=linux/arm64
```

Local images include `debian:trixie-slim` (linux/arm64, digest `sha256:a99cfc517144bc59b1978475ec53b46ecabec7e43635402ee5b77cc54cd1b20a`) and `wechatopenai/weknora-sandbox:main` (linux/arm64, image ID `sha256:69c14626ee0f92ef01756939ee600b763e5caade8b176607fa0e84e28eb917e3`). No local repository/tag matched Chromium, Chrome, Selenium, Playwright, or browser. A disposable shell in the sandbox image searched `/usr` and `/opt`; no `chromium`, `google-chrome`, `chromedriver`, or Firefox executable was found.

The host has `/Applications/Google Chrome.app`, but no `chromium`, `google-chrome`, `chromedriver`, or Firefox executable on PATH. A macOS Chrome binary is not a Linux container renderer. There is no host `apt-get`, `apt-cache`, or `dpkg`; only Homebrew is present. The checked-in Dockerfile needs Debian `chromium`, `chromium-driver`, Python/venv, and Selenium. Prior ARM64 BuildKit evidence records memory exhaustion while installing `libllvm19`.

The runner requires a digest-pinned prebuilt image and checks its exact RepoDigest and `linux/arm64` metadata. It creates the container with `--network none`, `--read-only`, UID/GID `10001:10001`, `--cap-drop ALL`, no-new-privileges, 1536 MiB memory, 2 CPUs, 128 PIDs, 128 MiB shared memory, only two tmpfs mounts, and no ports/bind mounts. It also requires `/usr/bin/chromium`, `chromedriver`, Python, and Selenium in the image.

Prior reports record that Docker Hub `selenium/standalone-chromium:latest` ARM64 digest `sha256:75ca4c0be92504568f0ba03bd2c5ef9b1098f29517179f7546955c30d838ac78` stalled for about 5.5 minutes without producing a local image, and a corrected GHCR pull reached layers but failed with `short read ... unexpected EOF`. No retry was performed here.

## Inference

The digest-pinned official Selenium image is the most reproducible next route because it supplies maintained ARM64 Chromium and ChromeDriver without repeating the memory-heavy Debian package build. It still needs a successful pull, local inspect, and runtime compatibility check under UID 10001, read-only root, 128 MiB /dev/shm, and `network none`. Selenium's normal Grid entrypoint must remain overridden as the runner specifies.

The preloaded sandbox image and macOS Chrome cannot substitute for a Linux/arm64 browser image based on current contents. No browser-level page-load, navigation-denial, screenshot, CDP, or same-bound-container evidence exists from this probe.

## Recommendation and acceptance boundary

Run the existing prebuilt path in a Linux/arm64 environment with reliable registry access, preferably using the GHCR digest above and retaining pull output plus `docker image inspect`. Then run the unchanged runner and retain browser/driver versions, container inspection, screenshot, and complete evidence validation. If GHCR repeats the short-read failure, try the recorded official Docker Hub ARM64 digest as a retrieval fallback; treat it as unverified until the exact image is local and passes the runner.

Keep `BrowserNavigationProtected` false and T14 blocked in the current environment. This note provides a verified environment blocker and reproducible next route; it does not promote T14 or establish browser/network attestation.

## Source pointers

- Approved acceptance and failure handling: `docs/plans/2026-09-23-craft-107-t14-brief.md`.
- Runner contract: `deploy/craft/render-boundary/run-probe.sh` and `deploy/craft/render-boundary/Dockerfile`.
- Digest/image research and prior pull results: `docs/plans/2026-09-23-craft-107-t14-browser-image-research.md`, `docs/plans/2026-09-23-craft-107-t14-render-boundary-recovery-report.md`, `docs/plans/2026-09-23-craft-107-t14-prebuilt-browser-report.md`.
- Browser proof design and limits: `docs/plans/2026-09-23-craft-107-t14-document-mechanism-proof-design.md` and `docs/plans/2026-09-23-craft-107-t14-render-proof-fix-review.md`.
