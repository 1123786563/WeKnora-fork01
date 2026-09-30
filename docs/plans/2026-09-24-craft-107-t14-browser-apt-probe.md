# T14 browser image feasibility probe (2026-09-24)

## Scope and identity

This is a read-only/disposable probe for T14. The reviewed image was not
modified, and no package download, install, build, or source change was made.

Command:

```sh
docker image inspect weknora-craft-runtime:1.18.4 \
  --format '{{.Id}} {{.Architecture}}/{{.Os}} {{json .RepoDigests}}'
```

Result: `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`
(`arm64/linux`), with digest
`weknora-craft-runtime@sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`.

## Verified facts

All commands below used `docker run --rm --platform linux/arm64 --user 0:0
--entrypoint sh weknora-craft-runtime:1.18.4 ...`; therefore each container was
disposable. The image is Debian GNU/Linux 12 (bookworm), `dpkg` architecture
`arm64`. Its signed sources are:

* `http://ftp.cn.debian.org/debian`, suites `bookworm bookworm-updates`, component `main`;
* `http://ftp.cn.debian.org/debian-security`, suite `bookworm-security`, component `main`;
* both use `/usr/share/keyrings/debian-archive-keyring.gpg`.

`apt-get update -qq` succeeded in the root disposable container.

`apt-cache policy` found all requested packages for arm64:

| Package | Candidate | Package installed size | Archive size |
|---|---:|---:|---:|
| chromium | 153.0.8010.52-1~deb12u1 (security) | 271,845 KiB | 69,882,312 B |
| chromium-driver | 153.0.8010.52-1~deb12u1 (security) | 23,021 KiB | 7,577,704 B |
| fonts-liberation | 1:1.07.4-11 | 2,093 KiB | 827,812 B |
| fonts-noto-color-emoji | 2.042-0+deb12u1 | 10,740 KiB | 9,893,872 B |
| fonts-dejavu-core | 2.37-6 (already installed) | 2,960 KiB | 1,067,728 B |

The `chromium-driver` package depends on the exact same Chromium version, so
the security candidates are version-aligned. Chromium's package metadata also
requires the usual GTK/graphics/audio/runtime libraries and `chromium-common`.

Dry-run command:

```sh
apt-get -s --no-install-recommends install \
  chromium chromium-driver fonts-liberation fonts-noto-color-emoji
```

Result: `126 newly installed`, `0 upgraded`, `0 to remove`; the simulated
closure includes `chromium-common`, GTK 3, Mesa/GBM, X11/Wayland, Pango,
fontconfig, audio libraries, dbus/systemd support, and related runtime
libraries. Recommended packages were not included. The primary package
payload alone is about 306 MiB installed (271,845 + 23,021 + 2,093 + 10,740
KiB), before `chromium-common` and the dependency closure; the real derivative
image should budget substantially more (roughly 0.7--1.0 GiB additional image
space) and verify storage limits before building.

The existing browser runner is Playwright, configured in
`apps/web/playwright.craft.config.ts`: one worker, Chromium-compatible browser
launch expected from Playwright, `CRAFT_WEB_URL`/`CRAFT_AUTH_STATE`, and
`test:craft:web` runs `pnpm --filter @weknora/web exec playwright test -c
playwright.craft.config.ts`. The T14 craft specs are `apps/web/e2e/craft-*.spec.ts`.

## Recommendation and disposable recipe

Package availability is sufficient for a test-only derivative image. A future
probe/build may use this recipe (do not retag or mutate the reviewed image):

```Dockerfile
FROM weknora-craft-runtime:1.18.4
USER root
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      chromium chromium-driver fonts-liberation fonts-noto-color-emoji \
 && rm -rf /var/lib/apt/lists/*
# Keep the base image entrypoint unless the runner explicitly overrides it.
```

Use the resulting test-only tag with the existing stack harness and Playwright
config; no application or test source change is indicated by this probe.
Pinning the package versions to the candidates above is recommended for a
reproducible acceptance run, because the configured repositories are rolling
bookworm/security metadata rather than an immutable snapshot.

## Limits

This probe did not build the derivative, launch Chromium, run T14, or verify
Playwright's bundled browser path against `/usr/bin/chromium`. It establishes
package availability and dependency closure only. A follow-up disposable build
must verify the actual executable path, sandbox flags/container privileges,
Playwright launch, and available disk/RAM.
