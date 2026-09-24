# T01/T19 E0 Linux image build probe

Date: 2026-09-24 (Asia/Shanghai)

## Scope and command

Read-only feasibility probe for the existing `docker/craft/Dockerfile` and
`docker/craft/opencode.lock.json`. No Dockerfile, lock, application, or test
files were changed. The probe used the native Docker `linux/arm64` platform on
the local `linux/arm64` Docker server, with the requested single bounded build:

```text
timeout 480 docker build --platform linux/arm64 --progress=plain \
  -t weknora-craft-runtime:1.18.4 \
  -f docker/craft/Dockerfile docker/craft
```

The full transient output was captured during the run at
`/tmp/weknora-craft-t01-linux-build.log`; the decisive excerpt is recorded
below.

## Verified facts

- Docker server reported version `29.4.0`, `linux/arm64`; builder instance was
  `orbstack` using the Docker driver.
- The lock is for OpenCode `1.18.4`, source commit
  `49c69c5ed3ccf706b61b3febb43c8aaff7f8325e`; its container target remains
  unpinned (`container_digest: null`, status `pending-verification`).
- The `opencode-fetch` stage reached completion. For `TARGETARCH=arm64`, the
  Dockerfile downloaded the v1.18.4 arm64 release, verified binary SHA-256
  `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`, and
  the in-build version probe printed exactly `1.18.4`.
- The final image was not produced. `docker images` showed no
  `weknora-craft-runtime:1.18.4` tag after the run, so there is no image digest
  or runnable route probe to report.
- The build reached runtime step `#10` (Debian packages for Python,
  LibreOffice, and fonts). It was canceled while fetching package
  `libxrender1`, after earlier Debian mirror `502 Bad Gateway` errors for
  `libtirpc-common`, `libtirpc3`, `libpython3.11-stdlib`, `libblas3`,
  `coinor-libclp1`, `dirmngr`, and `fontconfig-config`.

Decisive tail:

```text
#10 407.2 Get:74 ... libxext6 ...
#10 CANCELED
ERROR: failed to build: failed to solve: Canceled: context canceled
```

## Interpretation

The OpenCode arm64 fetch, digest check, and version check are feasible and
verified in the intermediate stage. The complete image build is currently
unverified because the large Debian runtime install was canceled; the
intermittent mirror `502` responses are an environmental failure signal. The
partial build may have populated builder cache layers, but no final tagged
image exists.

## Limits

This probe does not establish image isolation, credential absence at runtime,
network egress policy, server route availability, or protocol behavior. No
credentials, provider model calls, test databases, or ports were used.

## Follow-up attempt (single retry, 2026-09-24)

The authorized follow-up was run from the exact integration worktree after
printing both `pwd` and `git rev-parse --show-toplevel`; both resolved to
`/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.
Docker cache was reused and BuildKit parallelism was limited with
`BUILDKIT_MAX_PARALLELISM=1`:

```text
timeout 1200 docker build --platform linux/arm64 --progress=plain \
  -t weknora-craft-runtime:1.18.4 \
  -f docker/craft/Dockerfile docker/craft
```

The OpenCode fetch stage was cached, including the same arm64 digest and
`1.18.4` version verification. The runtime package step progressed further but
failed in `apt-get install` after fetching 175 MB. The remaining exact fetch
failures were all Debian mirror `502 Bad Gateway` responses:

```text
libsqlite3-0
libgfortran5
gnupg-l10n
gpg
gpg-agent
gpgsm
libxdmcp6
librtmp1
libmythes-1.2-0
libxslt1.1
E: Unable to fetch some archives, maybe run apt-get update or try with --fix-missing?
Dockerfile:104
ERROR: ... did not complete successfully: exit code: 100
```

No `weknora-craft-runtime:1.18.4` image or digest exists after this follow-up;
`docker images` returned no matching tag. This is the final permitted probe:
the remaining blocker is the transient/unreliable Debian package mirror during
the unmodified Dockerfile's runtime install, rather than the OpenCode binary
pin or the 1200-second time limit.
