# Craft reproducible sandbox image — OpenCode 1.18.4, non-root, read-only inputs.
#
# Build context: docker/craft (contains runtime-config.json; opencode.lock.json
# is the R01 read-only lock authority referenced by the pinned values below).
#   docker build -t weknora-craft-runtime:1.18.4 -f docker/craft/Dockerfile docker/craft
#
# Hardening contract (the image enforces what it can; the rest is the run
# contract the platform assembler must pass and R03's verification replays):
#   * non-root runtime user craft (UID/GID 10001);
#   * /workspace/inputs is root-owned 0555 and must be mounted read-only —
#     the OpenCode process can read staged material but never write it;
#   * /workspace/output is the only writable workspace tree (craft-owned);
#   * OpenCode data/config live under /home/craft and are declared VOLUMEs so
#     a sandbox restart keeps sessions and settings (persistence check);
#   * no credential of any kind is baked in: no ENV tokens, no model keys.
#     Model access is configured at runtime by the server-side gateway only;
#     platform long-lived keys and tenant credentials never enter the image
#     or the workspace;
#   * never run with -v /var/run/docker.sock, /var/lib/docker or a host $HOME
#     mounted — the image ships no docker CLI and expects none;
#   * no symlinks ship anywhere under /workspace (build assertion below);
#     inputs and the workspace root stay root-owned so the runtime user
#     cannot repoint them with a symlink of its own.
#
# Binary pinning: the R01 lock (docker/craft/opencode.lock.json) pins the
# darwin/arm64 probe digest 9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c
# 62edea2a8de3f98 and requires each linux arch to be hashed on its own. The
# per-arch build args below are those measured digests, taken from the same
# v1.18.4 release (source commit 49c69c5ed3ccf706b61b3febb43c8aaff7f8325e);
# the builder re-verifies the extracted binary AND the printed version, then
# refuses to produce an image on any mismatch.

ARG OPENCODE_VERSION=1.18.4

# ---------- Stage 1: fetch and verify the locked OpenCode binary ----------
FROM debian:bookworm-slim AS opencode-fetch
ARG TARGETARCH
ARG OPENCODE_VERSION
ARG OPENCODE_RELEASE_BASE=https://github.com/anomalyco/opencode/releases/download
# sha256 of the binary inside opencode-linux-x64.tar.gz (v1.18.4)
ARG OPENCODE_SHA256_AMD64=6ce6570e7db9a40e7bd3304ebdfff607920bde8cafd2eb5587bd7a26f89ba0b5
# sha256 of the binary inside opencode-linux-arm64.tar.gz (v1.18.4)
ARG OPENCODE_SHA256_ARM64=3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) arch=x64; digest="${OPENCODE_SHA256_AMD64}";; \
      arm64) arch=arm64; digest="${OPENCODE_SHA256_ARM64}";; \
      *) echo "unsupported TARGETARCH ${TARGETARCH}" >&2; exit 1;; \
    esac; \
    install -d /tmp/unpack; \
    curl -fsSL --http1.1 --retry 5 --retry-all-errors --retry-delay 3 \
      -o /tmp/opencode.tar.gz \
      "${OPENCODE_RELEASE_BASE}/v${OPENCODE_VERSION}/opencode-linux-${arch}.tar.gz"; \
    tar -xzf /tmp/opencode.tar.gz -C /tmp/unpack; \
    test -f /tmp/unpack/opencode; \
    echo "${digest}  /tmp/unpack/opencode" | sha256sum -c -; \
    install -m 0755 /tmp/unpack/opencode /usr/local/bin/opencode; \
    rm -rf /tmp/unpack /tmp/opencode.tar.gz; \
    # Run the version probe from the installed path with a writable HOME: the
    # Bun-compiled binary materializes its cache under HOME/TMPDIR and would
    # collide with a same-named temp file if probed in place.
    install -d /tmp/oc-probe; \
    HOME=/tmp/oc-probe /usr/local/bin/opencode --version | grep -Fx "${OPENCODE_VERSION}"

# ---------- Stage 2: minimal non-root runtime ----------
FROM debian:bookworm-slim AS runtime
ARG OPENCODE_VERSION

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# Dedicated non-root account; fixed ids so mounts can address it numerically.
RUN groupadd --system --gid 10001 craft \
 && useradd --system --uid 10001 --gid craft --home-dir /home/craft --shell /usr/sbin/nologin craft

# Workspace layout. inputs is root-owned 0555: readable by everyone, writable
# by no one but root — the platform stages material before the sandbox boots
# and mounts the tree read-only anyway. The workspace root itself stays
# root-owned so the runtime user cannot shadow inputs/ or output/ with
# symlinks or renames of its own.
RUN install -d -o root -g root -m 0755 /workspace \
 && install -d -o root -g root -m 0555 /workspace/inputs \
 && install -d -o craft -g craft -m 0755 /workspace/output \
 && install -d -o craft -g craft -m 0755 /home/craft/.local/share/opencode \
 && install -d -o craft -g craft -m 0755 /home/craft/.config/opencode \
 && install -d -o root -g root -m 0755 /etc/craft \
 && chown -R craft:craft /home/craft \
 && chmod 0755 /home/craft

# ---------- D02: spreadsheet toolchain (openpyxl + LibreOffice headless) ----
# The craft-spreadsheet skill creates and parses workbooks with openpyxl and
# recalculates them with LibreOffice headless: a written formula without a
# recalculated cached value never ships (SheetReady requires Recalculated).
# libreoffice-calc-nogui is the GUI-free Calc; fonts-noto-cjk (SIL OFL 1.1)
# covers CJK cell text. The resolved package versions of this set are
# recorded in docker/craft/spreadsheet-requirements.lock; apt's repository
# signature verification pins the debs at install time, and the build fails
# loudly if openpyxl or soffice do not run.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      python3 \
      python3-openpyxl \
      libreoffice-calc-nogui \
      fonts-noto-cjk \
 && rm -rf /var/lib/apt/lists/* \
 && python3 -c 'import openpyxl,sys;print("openpyxl",openpyxl.__version__,"on",sys.version.split()[0])' \
 && soffice --version

# ---------- D03: slides toolchain (python-pptx + LibreOffice Impress + poppler) ----
# The craft-slides skill writes decks with python-pptx, renders them to PDF
# with LibreOffice Impress headless and exports every page to an image with
# poppler's pdftocairo: a slide that was never rendered keeps the kind
# closed (SlidesReady requires every PageRef). Debian bookworm does not
# package python3-pptx, so the wheel is pinned BY SHA256 and installed
# --no-deps over Debian's own lxml/Pillow (python3-lxml arrives with the
# D02 openpyxl set; python3-pil is added here). The resolved versions are
# recorded in docker/craft/slides-requirements.lock; the build fails loudly
# if pptx, soffice (Impress) or pdftocairo do not run. python-pptx's own
# runtime deps that Debian also does not package (XlsxWriter for chart data,
# typing_extensions) are pinned the same way; lxml/Pillow come from Debian.
ARG PYTHON_PPTX_VERSION=1.0.2
ARG PYTHON_PPTX_SHA256=160838e0b8565a8b1f67947675886e9fea18aa5e795db7ae531606d68e785cba
# Full PyPI wheel URLs (path prefixes are version-specific, so they move with the pin).
ARG PYTHON_PPTX_URL=https://files.pythonhosted.org/packages/d9/4f/00be2196329ebbff56ce564aa94efb0fbc828d00de250b1980de1a34ab49/python_pptx-1.0.2-py3-none-any.whl
ARG XLSXWRITER_VERSION=3.2.0
ARG XLSXWRITER_SHA256=ecfd5405b3e0e228219bcaf24c2ca0915e012ca9464a14048021d21a995d490e
ARG XLSXWRITER_URL=https://files.pythonhosted.org/packages/a7/ea/53d1fe468e63e092cf16e2c18d16f50c29851242f9dd12d6a66e0d7f0d02/XlsxWriter-3.2.0-py3-none-any.whl
ARG TYPING_EXTENSIONS_VERSION=4.12.2
ARG TYPING_EXTENSIONS_SHA256=04e5ca0351e0f3f85c6853954072df659d0d13fac324d0072316b67d7794700d
ARG TYPING_EXTENSIONS_URL=https://files.pythonhosted.org/packages/26/9f/ad63fc0248c5379346306f8668cda6e2e2e9c95e01216d2b8ffd9ff037d0/typing_extensions-4.12.2-py3-none-any.whl
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      curl \
      python3-pip \
      python3-pil \
      libreoffice-impress-nogui \
      poppler-utils \
 && rm -rf /var/lib/apt/lists/* \
 && for w in "${PYTHON_PPTX_URL} ${PYTHON_PPTX_SHA256}" "${XLSXWRITER_URL} ${XLSXWRITER_SHA256}" "${TYPING_EXTENSIONS_URL} ${TYPING_EXTENSIONS_SHA256}"; do \
      set -- ${w}; \
      curl -fsSL --http1.1 --retry 5 --retry-all-errors --retry-delay 3 -o "/tmp/$(basename "$1")" "$1"; \
      echo "$2  /tmp/$(basename "$1")" | sha256sum -c -; \
      python3 -m pip install --break-system-packages --no-deps --no-cache-dir "/tmp/$(basename "$1")"; \
      rm -f "/tmp/$(basename "$1")"; \
    done \
 && python3 -c 'import pptx, xlsxwriter, lxml.etree, PIL, sys; from pptx.chart.data import CategoryChartData; print("python-pptx", pptx.__version__, "+ XlsxWriter", xlsxwriter.__version__, "on", sys.version.split()[0])' \
 && soffice --version \
 && pdftocairo -v 2>&1 | sed -n 1p

COPY --from=opencode-fetch /usr/local/bin/opencode /usr/local/bin/opencode
COPY runtime-config.json /etc/craft/runtime-config.json
# COPY keeps the source file mode; force the world-readable mode so the
# non-root runtime user can always read the runtime contract.
RUN chmod 0644 /etc/craft/runtime-config.json

# OpenCode global config derived from /etc/craft/runtime-config.json's
# permission policy. Keep in sync with that file's "permissions" section:
# default for everything unlisted is ask, no broad always-allow, webfetch
# denied, autoupdate off for reproducibility.
RUN printf '%s\n' \
    '{' \
    '  "$schema": "https://opencode.ai/config.json",' \
    '  "autoupdate": false,' \
    '  "permission": {' \
    '    "edit": "ask",' \
    '    "bash": "ask",' \
    '    "webfetch": "deny"' \
    '  }' \
    '}' > /home/craft/.config/opencode/opencode.json \
 && chown craft:craft /home/craft/.config/opencode/opencode.json \
 && chmod 0644 /home/craft/.config/opencode/opencode.json

# Build-time symlink ban: the shipped workspace tree must contain no symlinks
# at all, so nothing can smuggle a traversal through this image. (Runtime
# input staging is content-addressed single files; the read-only mount plus
# root-owned parents keep the invariant afterwards.)
RUN test -z "$(find /workspace -type l -print -quit)"

ENV HOME=/home/craft \
    XDG_CONFIG_HOME=/home/craft/.config \
    XDG_DATA_HOME=/home/craft/.local/share

# Persistent OpenCode data and config survive sandbox restarts via these
# volumes; everything else in the container is disposable by design.
VOLUME ["/home/craft/.local/share/opencode", "/home/craft/.config/opencode"]

USER 10001:10001
WORKDIR /workspace/output
EXPOSE 4096

# The executor dials this server inside the sandbox network namespace only;
# it is never published to the host network.
ENTRYPOINT ["/usr/local/bin/opencode"]
CMD ["serve", "--hostname", "0.0.0.0", "--port", "4096"]
