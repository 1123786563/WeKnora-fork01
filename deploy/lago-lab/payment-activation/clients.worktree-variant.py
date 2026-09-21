#!/usr/bin/env python3
"""Redaction-safe clients and lab-env generation for the Lago T02 lab.

stdlib only. Three concerns live here:

- ``generate_lab_env`` / ``write_lab_env``: the secret set for the lab's own
  isolated Compose project (``weknora-lago-74``), including the one-shot seed
  values that mint the operator account and organization API key on first
  ``up``. Secrets are generated, never hard-coded, and written mode 600.
- HTTP clients: ``LagoRestClient`` (bearer auth, JSON; the credential is only
  ever sent to the configured origin and off-origin redirects are refused
  rather than followed), ``lago_graphql_login`` (operator JWT), and
  ``StripeTestClient`` (test-mode-only Stripe API; live keys are refused at
  construction).
- ``sanitize``: deep redaction used on every value before it may reach a
  report or log; strips Bearer tokens, Stripe key shapes, JWT shapes, and any
  caller-supplied canary secrets.

No secret value is ever part of ``repr``/``str`` of any object defined here.
"""

from __future__ import annotations

import argparse
import copy
import json
import os
import re
import secrets
import stat
import subprocess
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlsplit
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener

# --- lab isolation constants (Ticket #74 plan: Global Constraints) -----------

COMPOSE_PROJECT_NAME = "weknora-lago-74"
LAGO_API_PORT = "48891"
LAGO_FRONT_PORT = "48892"
LAGO_API_URL = f"http://127.0.0.1:{LAGO_API_PORT}"
LAGO_FRONT_URL = f"http://127.0.0.1:{LAGO_FRONT_PORT}"

# Official Lago v1.53.0 sample .env defaults that generated secrets must never
# equal (mirrors deploy/lago/lago.sh).
SAMPLE_DEFAULTS = {
    "POSTGRES_PASSWORD": "changeme",
    "SECRET_KEY_BASE": "your-secret-key-base-hex-64",
    "LAGO_ENCRYPTION_PRIMARY_KEY": "your-encryption-primary-key",
    "LAGO_ENCRYPTION_DETERMINISTIC_KEY": "your-encryption-deterministic-key",
    "LAGO_ENCRYPTION_KEY_DERIVATION_SALT": "your-encryption-derivation-salt",
}

# Stripe keys must be test mode; live keys are refused outright.
STRIPE_TEST_PREFIXES = ("sk_test_", "rk_test_")

# Secret shapes that sanitize() always redacts, regardless of caller input.
_SECRET_PATTERNS = (
    re.compile(r"Bearer\s+[A-Za-z0-9._~+/=-]{8,}"),
    re.compile(r"\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{8,}\b"),
    re.compile(r"eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{8,}"),
)
_REDACTED = "***REDACTED***"


class LabError(RuntimeError):
    """A lab precondition failed (bad env file, refused overwrite, ...)."""


class RefusedLiveKey(ValueError):
    """A non-test-mode Stripe key (or empty key) was passed to StripeTestClient."""


class LoginError(LabError):
    """GraphQL login failed. Never carries the password or any token."""


class OffOriginRedirect(LabError):
    """A redirect tried to move the request off the configured origin."""

    def __init__(self, target):
        self.target = target
        super().__init__(f"refusing off-origin redirect to {target!r}")


def _redact_text(text, extra_secrets=()):
    for secret in extra_secrets:
        if secret:
            text = text.replace(secret, _REDACTED)
    for pattern in _SECRET_PATTERNS:
        text = pattern.sub(_REDACTED, text)
    return text


def sanitize(value, extra_secrets=()):
    """Return a deep copy of value with every secret shape redacted."""
    if isinstance(value, str):
        return _redact_text(value, extra_secrets)
    if isinstance(value, dict):
        return {
            _redact_text(str(k), extra_secrets) if isinstance(k, str) else k: sanitize(v, extra_secrets)
            for k, v in value.items()
        }
    if isinstance(value, (list, tuple)):
        redacted = [sanitize(item, extra_secrets) for item in value]
        return type(value)(redacted) if isinstance(value, tuple) else redacted
    return copy.deepcopy(value)


# --- lab.env generation ------------------------------------------------------


def _generate_rsa_key():
    """Generate a base64-encoded RSA private key via openssl (offline)."""
    key = subprocess.run(
        ["openssl", "genrsa", "2048"],
        capture_output=True,
        check=True,
    ).stdout
    encoded = subprocess.run(
        ["openssl", "base64", "-A"],
        input=key,
        capture_output=True,
        check=True,
    ).stdout.decode("ascii").strip()
    if not encoded:
        raise LabError("openssl produced an empty RSA key")
    return encoded


def generate_lab_env(rsa_private_key=None):
    """Build the full lab env mapping: isolation pins, secrets, seed values."""
    if rsa_private_key is None:
        rsa_private_key = _generate_rsa_key()
    if not rsa_private_key:
        raise LabError("LAGO_RSA_PRIVATE_KEY must not be empty")
    return {
        "COMPOSE_PROJECT_NAME": COMPOSE_PROJECT_NAME,
        "LAGO_API_PORT": LAGO_API_PORT,
        "LAGO_FRONT_PORT": LAGO_FRONT_PORT,
        "LAGO_API_URL": LAGO_API_URL,
        "LAGO_FRONT_URL": LAGO_FRONT_URL,
        "POSTGRES_PASSWORD": secrets.token_urlsafe(24),
        "SECRET_KEY_BASE": secrets.token_hex(32),
        "LAGO_ENCRYPTION_PRIMARY_KEY": secrets.token_urlsafe(32),
        "LAGO_ENCRYPTION_DETERMINISTIC_KEY": secrets.token_urlsafe(32),
        "LAGO_ENCRYPTION_KEY_DERIVATION_SALT": secrets.token_urlsafe(16),
        "LAGO_RSA_PRIVATE_KEY": rsa_private_key,
        # One-shot seed (compose migrate service): operator account + API key
        # are minted with exactly these local throwaway values on first `up`.
        "LAGO_CREATE_ORG": "true",
        "LAGO_ORG_USER_EMAIL": "ops-t02@weknora.local",
        "LAGO_ORG_USER_PASSWORD": secrets.token_urlsafe(18),
        "LAGO_ORG_NAME": "WeKnora T02 Lab",
        "LAGO_ORG_API_KEY": secrets.token_urlsafe(32),
    }


def validate_lab_env(env):
    """Reject env mappings that equal Lago sample defaults or violate pins."""
    for key, sample in SAMPLE_DEFAULTS.items():
        value = env.get(key, "")
        if not value:
            raise LabError(f"{key} is empty -- run lab.sh init")
        if value == sample:
            raise LabError(f"{key} still equals the official Lago sample default")
    if not env.get("LAGO_RSA_PRIVATE_KEY"):
        raise LabError("LAGO_RSA_PRIVATE_KEY is empty")
    license_value = env.get("LAGO_LICENSE", "")
    if license_value:
        raise LabError(
            "LAGO_LICENSE must stay empty (Community boundary; the lab never "
            "unlocks Premium features)"
        )
    for name, port_key, url_key in (
        ("LAGO_API", "LAGO_API_PORT", "LAGO_API_URL"),
        ("LAGO_FRONT", "LAGO_FRONT_PORT", "LAGO_FRONT_URL"),
    ):
        url = env.get(url_key, "")
        if url and url.rsplit(":", 1)[-1] != env.get(port_key, ""):
            raise LabError(
                f"{url_key} '{url}' does not point at the configured {port_key} -- fix lab.env"
            )


def write_lab_env(path, env=None):
    """Write the lab env file mode 600, refusing to overwrite an existing one."""
    if env is None:
        env = generate_lab_env()
    validate_lab_env(env)
    path = os.fspath(path)
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    fd = os.open(path, flags, 0o600)
    try:
        os.fchmod(fd, 0o600)  # survive umask differences
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            fd = -1
            handle.write("# Generated by lab.sh init -- local secrets, never commit.\n")
            for key, value in env.items():
                handle.write(f'{key}="{value}"\n')
    finally:
        if fd >= 0:
            os.close(fd)
    os.chmod(path, 0o600)


# --- HTTP helpers -------------------------------------------------------------


class _OriginBoundRedirectHandler(HTTPRedirectHandler):
    """Follow redirects only within the configured origin."""

    def __init__(self, origin):
        super().__init__()
        self._origin = origin  # (scheme, netloc)

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        target = urlsplit(newurl)
        if (target.scheme, target.netloc) != self._origin:
            raise OffOriginRedirect(f"{target.scheme}://{target.netloc}")
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def _proxyless_opener(origin=None):
    """An opener that ignores system/env proxies (loopback origins only)."""
    handlers = [ProxyHandler({})]
    if origin is not None:
        handlers.append(_OriginBoundRedirectHandler(origin))
    return build_opener(*handlers)


class LagoRestClient:
    """JSON REST client for the Lago API. Bearer credential is origin-bound."""

    def __init__(self, base_url, api_key, timeout=30):
        self._base_url = str(base_url).rstrip("/")
        self._api_key = api_key
        self._timeout = timeout
        parts = urlsplit(self._base_url)
        self._origin = (parts.scheme, parts.netloc)
        self._opener = _proxyless_opener(self._origin)

    def __repr__(self):
        return f"LagoRestClient(base_url={self._base_url!r})"

    __str__ = __repr__

    def request(self, method, path, payload=None):
        """One JSON call. Returns (http_status, parsed_body_or_None).

        Non-2xx statuses are returned, not raised, so callers can record the
        real contract. Transport failures raise OSError; off-origin redirects
        raise OffOriginRedirect.
        """
        url = path if path.startswith("http") else f"{self._base_url}{path}"
        headers = {"Accept": "application/json"}
        body = None
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        target = urlsplit(url)
        if (target.scheme, target.netloc) == self._origin and self._api_key:
            headers["Authorization"] = f"Bearer {self._api_key}"
        request = Request(url, data=body, headers=headers, method=method)
        try:
            with self._opener.open(request, timeout=self._timeout) as response:
                status = response.status
                raw = response.read()
        except HTTPError as error:
            raw = error.read()
            return error.code, _parse_json_body(raw)
        if not raw:
            return status, None
        return status, _parse_json_body(raw)

    def get(self, path):
        return self.request("GET", path)

    def post(self, path, payload):
        return self.request("POST", path, payload=payload)

    def put(self, path, payload):
        return self.request("PUT", path, payload=payload)

    def delete(self, path):
        return self.request("DELETE", path)


def _parse_json_body(raw):
    try:
        return json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return {"_raw": _redact_text(raw.decode("utf-8", errors="replace"))}


def lago_graphql_login(base_url, email, password, timeout=30):
    """Log the operator in via GraphQL loginUser; return the JWT string.

    Raises LoginError on any failure. The exception text never contains the
    password, the email, or any token.
    """
    base_url = str(base_url).rstrip("/")
    query = (
        "mutation LoginUser($input: LoginUserInput!) {"
        " loginUser(input: $input) { token } }"
    )
    payload = {
        "query": query,
        "variables": {"input": {"email": email, "password": password}},
    }
    request = Request(
        f"{base_url}/graphql",
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json", "Accept": "application/json"},
        method="POST",
    )
    try:
        with _proxyless_opener().open(request, timeout=timeout) as response:
            status = response.status
            body = json.loads(response.read().decode("utf-8"))
    except HTTPError as error:
        raise LoginError(f"graphql login returned HTTP {error.code}") from None
    except (OSError, URLError) as error:
        raise LoginError("graphql login request failed") from error
    except ValueError:
        raise LoginError("graphql login response was not valid JSON") from None
    if status != 200:
        raise LoginError(f"graphql login returned HTTP {status}")
    data = body.get("data") if isinstance(body, dict) else None
    token = data.get("loginUser", {}).get("token") if isinstance(data, dict) else None
    if not isinstance(token, str) or not token:
        errors = body.get("errors") if isinstance(body, dict) else None
        detail = ""
        if isinstance(errors, list) and errors and isinstance(errors[0], dict):
            detail = f": {_redact_text(str(errors[0].get('message', '')))}"
        raise LoginError(f"graphql login failed{detail}")
    return token


class StripeTestClient:
    """Minimal Stripe v1 client that only ever accepts test-mode keys."""

    def __init__(self, api_key, base_url="https://api.stripe.com", timeout=30):
        key = api_key or ""
        if not key.startswith(STRIPE_TEST_PREFIXES):
            raise RefusedLiveKey(
                "refusing Stripe key: only sk_test_/rk_test_ keys are allowed"
            )
        self._api_key = key
        self._base_url = str(base_url).rstrip("/")
        self._timeout = timeout
        self._opener = _proxyless_opener()

    def __repr__(self):
        return f"StripeTestClient(base_url={self._base_url!r})"

    __str__ = __repr__

    def _form(self, method, path, params=None):
        body = urlencode(params or {}).encode("utf-8")
        request = Request(
            f"{self._base_url}{path}",
            data=body if method != "DELETE" else None,
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/x-www-form-urlencoded",
                "Stripe-Account": "",
            },
            method=method,
        )
        if method == "DELETE" and params:
            request.selector = f"{path}?{urlencode(params)}"
        try:
            with self._opener.open(request, timeout=self._timeout) as response:
                status = response.status
                raw = response.read()
        except HTTPError as error:
            raw = error.read()
            return error.code, _parse_json_body(raw)
        if not raw:
            return status, None
        return status, _parse_json_body(raw)

    def create_customer(self, description=None, metadata=None):
        params = {}
        if description:
            params["description"] = description
        for key, value in (metadata or {}).items():
            params[f"metadata[{key}]"] = value
        status, body = self._form("POST", "/v1/customers", params)
        if status != 200 or not isinstance(body, dict) or not body.get("id"):
            raise LabError(f"stripe create_customer failed (HTTP {status})")
        return body

    def attach_payment_method(self, payment_method_id, customer_id):
        status, body = self._form(
            "POST", f"/v1/payment_methods/{payment_method_id}/attach",
            {"customer": customer_id},
        )
        if status != 200 or not isinstance(body, dict) or not body.get("id"):
            raise LabError(f"stripe attach payment method failed (HTTP {status})")
        return body

    def set_default_payment_method(self, customer_id, payment_method_id):
        status, body = self._form(
            "POST", f"/v1/customers/{customer_id}",
            {"invoice_settings[default_payment_method]": payment_method_id},
        )
        if status != 200 or not isinstance(body, dict) or not body.get("id"):
            raise LabError(f"stripe set default payment method failed (HTTP {status})")
        return body

    def delete_customer(self, customer_id):
        """Best-effort delete; returns True when Stripe confirms deletion."""
        try:
            status, body = self._form("DELETE", f"/v1/customers/{customer_id}")
        except OSError:
            return False
        return status == 200 and isinstance(body, dict) and body.get("deleted") is True


def _main(argv=None):
    parser = argparse.ArgumentParser(description="lab.env generation for the T02 lab")
    parser.add_argument("gen-env", help="generate the lab env file")
    parser.add_argument("--output", required=True, help="target path (must not exist)")
    args = parser.parse_args(argv)
    rsa_key = os.environ.get("LAB_RSA_KEY_INPUT") or None
    try:
        write_lab_env(args.output, generate_lab_env(rsa_private_key=rsa_key))
    except LabError as error:
        print(f"lab gen-env: {error}", file=sys.stderr)
        return 1
    print(f"wrote {args.output} (mode 600) -- secrets are local-only")
    return 0


if __name__ == "__main__":
    sys.exit(_main())
