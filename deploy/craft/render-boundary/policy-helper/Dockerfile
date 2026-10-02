# Base is pinned to python:3.12-alpine@sha256:4c47124a8391cb7a9f571164147d154777cf012a4ece5f86097130d7a4478111
# (see build.sh for the digest-pinned form). The digest-by-digest reference is
# resolved via the local tag because the configured registry mirror rejects
# manifest HEAD requests for it; the local tag's image ID equals the pinned digest.
FROM python:3.12-alpine
LABEL org.opencontainers.image.title="T14 isolated renderer policy helper" \
    org.opencontainers.image.description="Short-lived NET_ADMIN-only nftables/iproute2 helper for disposable renderer namespaces" \
    org.opencontainers.image.source="local:deploy/craft/render-boundary/policy-helper" \
    org.opencontainers.image.base.name="python:3.12-alpine@sha256:4c47124a8391cb7a9f571164147d154777cf012a4ece5f86097130d7a4478111"
RUN apk add --no-cache nftables=1.1.6-r1 iproute2=7.0.0-r0 \
    && apk list -I nftables iproute2 > /usr/share/policy-helper-package-provenance.txt
COPY helper.py /policy-helper.py
ENTRYPOINT ["timeout", "30", "python3", "/policy-helper.py"]
