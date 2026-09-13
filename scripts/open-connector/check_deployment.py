#!/usr/bin/env python3
"""Static validator for the private open-connector compose stack (T16).

Usage:
    python3 scripts/open-connector/check_deployment.py <compose-config.json>

The input is the JSON produced by
    docker compose -f docker/compose.open-connector.yaml config --format json
written to an ISOLATED temp file (ruling 8: the parsed document may contain
secret-adjacent interpolation, so it is never echoed, only validated).

Checks (T16 brief):
  plan-sketch core   missing service / published private port errors;
  ruling 9 (R9)      pre-built images must be sha256-digest pinned and never
                     tip/latest; the runtime image may not be replaced by a
                     build section;
  ruling 2           the connector-admin-token secret is MANDATORY on
                     open-connector and connector-control (T01 evidence: a
                     zero-auth runtime is fully open) and FORBIDDEN on every
                     other service (the API process must never hold it);
  ruling 7           the connector-secret-key secret (secret-sink encryption
                     at rest) is required on the control worker;
  ruling 3           read-only rootfs, non-root user and a healthcheck for
                     the app services, healthcheck for the DB;
  ruling 4           the runtime keeps controlled egress: attached only to
                     internal networks is rejected (OAuth would break).

Exit codes: 0 = valid, 2 = validation errors, 1 = usage/IO error.
Python 3.9 stdlib only.
"""

import json
import sys

CORE_SERVICES = ("open-connector", "connector-db", "connector-control")
APP_SERVICES = ("open-connector", "connector-control")
ADMIN_SECRET = "connector-admin-token"
ADMIN_SECRET_ALLOWED = frozenset(("open-connector", "connector-control"))
ENCRYPTION_KEY_SECRET = "connector-secret-key"
FORBIDDEN_TAG_MARKERS = (":latest", ":tip")
DIGEST_MARKER = "@sha256:"


def _secret_sources(service):
    """Normalized secret source names of one service (long or short syntax)."""
    names = set()
    for entry in service.get("secrets") or []:
        if isinstance(entry, str):
            if entry:
                names.add(entry)
        elif isinstance(entry, dict):
            source = entry.get("source") or entry.get("target")
            if source:
                names.add(source)
    return names


def _service_network_names(service):
    """Network names a service is attached to (list or map syntax)."""
    declared = service.get("networks")
    if declared is None:
        return None
    if isinstance(declared, list):
        names = []
        for entry in declared:
            if isinstance(entry, str):
                names.append(entry)
            elif isinstance(entry, dict) and entry.get("name"):
                names.append(entry["name"])
        return names
    if isinstance(declared, dict):
        return list(declared.keys())
    return []


def _image_ref(service):
    image = service.get("image")
    return image if isinstance(image, str) else None


def _has_digest(ref):
    return DIGEST_MARKER in ref


def _has_forbidden_tag(ref):
    lowered = ref.lower()
    return any(marker in lowered for marker in FORBIDDEN_TAG_MARKERS)


def _is_non_root_user(user):
    if not isinstance(user, str) or not user.strip():
        return False
    first = user.split(":", 1)[0].strip()
    return first not in ("", "0", "root")


def _validate_image(name, service, errors):
    """R9 digest discipline: pre-built images digest-pinned, never tip/latest."""
    image = _image_ref(service)
    has_build = bool(service.get("build"))
    if name == "open-connector":
        # The runtime deploys ONLY the R9-pinned digest image; a build section
        # would silently swap the proven artifact for an unpinned one.
        if has_build:
            errors.append(
                "open-connector: a build section is forbidden; deploy the "
                "digest-pinned image (R9) built from the pinned upstream source"
            )
        if image is None:
            errors.append("open-connector: the digest-pinned image is required")
            return
        if _has_forbidden_tag(image):
            errors.append(
                "open-connector: image %s uses a mutable tag; :latest/:tip are "
                "forbidden, pin by sha256 digest (R9)" % image.split("@")[0]
            )
        if not _has_digest(image):
            errors.append(
                "open-connector: image %s is not sha256-digest pinned; pin the "
                "locally built digest (R9: ghcr tip is mutable and unproven)" % image
            )
        return
    if image is None:
        if not has_build:
            errors.append("%s: an image (or build) declaration is required" % name)
        return
    if _has_forbidden_tag(image):
        errors.append(
            "%s: image %s uses a mutable tag; :latest/:tip are forbidden, "
            "pin by sha256 digest (R9)" % (name, image.split("@")[0])
        )
    if has_build:
        # Locally composed builds (pinned source tree) may use a local tag;
        # the forbidden-tag check above still applies.
        return
    if not _has_digest(image):
        errors.append(
            "%s: image %s is not sha256-digest pinned; pre-built images must "
            "pin an immutable digest (R9: ghcr tip is mutable and unproven)" % (name, image)
        )


def _validate_admin_secret_scoping(services, errors):
    """ADMIN_TOKEN mandatory (ruling 2) and scoped to runtime + control only."""
    for name in ("open-connector", "connector-control"):
        svc = services.get(name) or {}
        if ADMIN_SECRET not in _secret_sources(svc):
            errors.append(
                "%s: the %s secret is MANDATORY (T01 evidence: a zero-auth "
                "runtime is fully open; provider proxy disabled + management "
                "auth forced on)" % (name, ADMIN_SECRET)
            )
    for name, svc in services.items():
        if name in ADMIN_SECRET_ALLOWED:
            continue
        if ADMIN_SECRET in _secret_sources(svc or {}):
            errors.append(
                "%s: must NOT mount %s (only open-connector and "
                "connector-control may hold the admin credential; API "
                "processes read back restricted tokens only)" % (name, ADMIN_SECRET)
            )


def _validate_encryption_key(services, errors):
    """The control worker needs the sink encryption key (ruling 7)."""
    svc = services.get("connector-control") or {}
    if ENCRYPTION_KEY_SECRET not in _secret_sources(svc):
        errors.append(
            "connector-control: the %s secret is required (secret sink is "
            "encrypted at rest; the writer must hold the key)" % ENCRYPTION_KEY_SECRET
        )


def _validate_hardening(services, errors):
    for name in APP_SERVICES:
        svc = services.get(name) or {}
        if svc.get("read_only") is not True:
            errors.append(
                "%s: read_only must be true (writable state belongs on "
                "volumes/tmpfs, not the root filesystem)" % name
            )
        if not _is_non_root_user(svc.get("user")):
            errors.append(
                "%s: must run as a non-root user (got %r)" % (name, svc.get("user"))
            )
    for name in CORE_SERVICES:
        svc = services.get(name) or {}
        if not svc.get("healthcheck"):
            errors.append("%s: a healthcheck declaration is required" % name)


# network_mode values compatible with the private zero-publish posture.
# "host" publishes everything on the host stack; "container:<id>" shares
# another container's stack; "none" removes networking - none of these can
# coexist with the mandated private-bridge topology (hardening QF-02).
_ALLOWED_NETWORK_MODES = frozenset((None, "", "default", "bridge"))


def _validate_network_mode(name, service, errors):
    mode = service.get("network_mode")
    if mode in _ALLOWED_NETWORK_MODES:
        return
    errors.append(
        "%s: network_mode %r would silently bypass the private zero-publish "
        "posture (host/container/none break the bridge isolation); remove it "
        "and attach the private networks instead" % (name, mode)
    )


def _validate_networks(networks, services, errors):
    """No published ports (sketch core) + controlled egress (ruling 4)."""
    svc = services.get("open-connector")
    if not svc:
        return
    declared = svc.get("networks")
    if declared is not None and not isinstance(declared, (list, dict)):
        errors.append("open-connector: networks must be a list of network names or a mapping")
        return
    # T17 edge closure (T16 residual): validate the DECLARED entry shapes
    # BEFORE filtering — _service_network_names silently drops malformed
    # entries (non-strings, mappings without "name"), which would let a
    # broken attachment pass validation unnoticed.
    if isinstance(declared, list):
        for entry in declared:
            if not ((isinstance(entry, str) and entry) or (isinstance(entry, dict) and entry.get("name"))):
                errors.append("open-connector: networks entries must be network names or mappings")
                return
    attached = _service_network_names(svc)
    if attached is None:
        # No explicit networks: compose attaches the default bridge, which is
        # not internal - controlled egress works.
        return
    for entry in attached:
        if not isinstance(entry, str) or not entry:
            errors.append("open-connector: networks entries must be network names or mappings")
            return
    internal_only = bool(attached) and all(
        (networks.get(n) or {}).get("internal") for n in attached
    )
    if internal_only:
        errors.append(
            "open-connector: attached only to internal networks; the runtime "
            "needs controlled egress for OAuth provider flows (bare "
            "internal:true would break OAuth)"
        )


def validate_compose(config):
    """Validate a docker compose config document; return the error list.

    The plan-sketch core is preserved verbatim (missing service / published
    private port); the extensions only add errors, never mute core ones.
    Malformed shapes (non-mapping documents, scalar service entries) are
    reported as friendly errors instead of raising (hardening QF-03).
    """
    errors = []
    if not isinstance(config, dict):
        return ["config: top-level compose document must be a mapping"]
    services = config.get("services", {})
    if not isinstance(services, dict):
        errors.append("services: must be a mapping of service name to service definition")
        services = {}
    for name, svc in services.items():
        if not isinstance(svc, dict):
            errors.append("%s: service definition must be a mapping" % name)
    services = {n: s for n, s in services.items() if isinstance(s, dict)}
    networks = config.get("networks")
    if networks is None:
        networks = {}
    if not isinstance(networks, dict):
        errors.append("networks: must be a mapping of network name to definition")
        networks = {}

    for name in ("open-connector", "connector-db", "connector-control"):
        svc = services.get(name)
        if not svc:
            errors.append("missing service: " + name)
        elif svc.get("ports"):
            errors.append("published private service: " + name)

    for name, svc in services.items():
        _validate_image(name, svc, errors)
        _validate_network_mode(name, svc, errors)
    _validate_admin_secret_scoping(services, errors)
    _validate_encryption_key(services, errors)
    _validate_hardening(services, errors)
    _validate_networks(networks, services, errors)
    return errors


def main(argv):
    if len(argv) != 2:
        print("usage: check_deployment.py <compose-config.json>", file=sys.stderr)
        return 1
    try:
        with open(argv[1]) as fh:
            config = json.load(fh)
    except (OSError, ValueError) as exc:
        # Never include file content in the error; path + reason only.
        print("cannot read config %s: %s" % (argv[1], exc), file=sys.stderr)
        return 1
    errors = validate_compose(config)
    if errors:
        for err in errors:
            print("DEPLOYMENT INVALID: %s" % err)
        return 2
    print("DEPLOYMENT OK")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
