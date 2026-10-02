#!/bin/sh
# Rebuilds craft-t14-policy-helper:2026-09-24 from the pinned base digest.
# The original Dockerfile bytes were not recoverable (no image layer, BuildKit
# cache, or worktree copy); this reproduces the four in-image layers exactly as
# recorded by `docker history` of sha256:c0cfa88b4db1...bf1f2bb.
set -eu

tag="craft-t14-policy-helper:2026-09-24"
base="python:3.12-alpine@sha256:4c47124a8391cb7a9f571164147d154777cf012a4ece5f86097130d7a4478111"

docker build -t "$tag" - <<EOF
FROM $base
LABEL org.opencontainers.image.title="T14 isolated renderer policy helper" \
    org.opencontainers.image.description="Short-lived NET_ADMIN-only nftables/iproute2 helper for disposable renderer namespaces" \
    org.opencontainers.image.source="local:deploy/craft/render-boundary/policy-helper" \
    org.opencontainers.image.base.name="$base"
RUN apk add --no-cache nftables=1.1.6-r1 iproute2=7.0.0-r0 \
    && apk list -I nftables iproute2 > /usr/share/policy-helper-package-provenance.txt
COPY helper.py /policy-helper.py
ENTRYPOINT ["timeout", "30", "python3", "/policy-helper.py"]
EOF
