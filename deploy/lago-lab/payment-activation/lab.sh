#!/usr/bin/env bash
# Lifecycle for the T02 payment-activation lab stack: an isolated instance of
# the #73-pinned Lago Community v1.53.0 stack (own Compose project, ports,
# volumes; never touches the weknora-lago (#73) or OpenMeter projects).
#
# Usage: ./deploy/lago-lab/payment-activation/lab.sh init|up|down|status|config
#
#   init    generate lab.env (random secrets + one-shot seed values, mode 600)
#   up      validate secrets, then start the isolated stack (up -d --wait)
#   down    stop the lab stack (named data volumes are preserved)
#   status  health snapshot JSON for the lab project only
#   config  resolved Compose config with secrets redacted
#
# The lab reuses deploy/lago/compose.yaml read-only; all state lives under the
# weknora-lago-74 Compose project (volumes weknora-lago-74_lago_*).

set -euo pipefail

LAB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$LAB_DIR/../../.." && pwd)"
COMPOSE_FILE="$REPO_DIR/deploy/lago/compose.yaml"
ENV_FILE="${LAB_ENV_FILE:-$LAB_DIR/lab.env}"
readonly COMPOSE_PROJECT="weknora-lago-74"

die() {
  echo "lab.sh: $*" >&2
  exit 1
}

usage() {
  sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  exit 2
}

# The single scoped entry point to the pinned stack: every invocation carries
# the lab compose file, the lab env file, and the lab project name.
compose() {
  docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$COMPOSE_PROJECT" "$@"
}

env_value() {
  local key="$1" file="${2:-$ENV_FILE}" line
  line="$(grep -E "^${key}=" "$file" | tail -n 1 || true)"
  line="${line#*=}"
  line="${line#\"}"
  printf '%s' "${line%\"}"
}

ensure_env() {
  [[ -f "$ENV_FILE" ]] || die "missing $ENV_FILE -- run lab.sh init first"
}

check_secret_differs() {
  local key="$1" sample="$2" value
  value="$(env_value "$key")"
  [[ -n "$value" ]] || die "$key is empty -- run lab.sh init"
  [[ "$value" != "$sample" ]] || die "$key still equals the official Lago sample default -- run lab.sh init"
}

check_secrets() {
  ensure_env
  check_secret_differs POSTGRES_PASSWORD "changeme"
  check_secret_differs SECRET_KEY_BASE "your-secret-key-base-hex-64"
  check_secret_differs LAGO_ENCRYPTION_PRIMARY_KEY "your-encryption-primary-key"
  check_secret_differs LAGO_ENCRYPTION_DETERMINISTIC_KEY "your-encryption-deterministic-key"
  check_secret_differs LAGO_ENCRYPTION_KEY_DERIVATION_SALT "your-encryption-derivation-salt"
  [[ -n "$(env_value LAGO_RSA_PRIVATE_KEY)" ]] || die "LAGO_RSA_PRIVATE_KEY is empty -- run lab.sh init"
  [[ -z "$(env_value LAGO_LICENSE)" ]] || die "LAGO_LICENSE must stay empty (Community boundary; the lab never unlocks Premium features)"
}

check_port_url_consistency() {
  local name port url url_port
  for name in LAGO_API LAGO_FRONT; do
    port="$(env_value "${name}_PORT")"
    url="$(env_value "${name}_URL")"
    [[ -n "$port" && -n "$url" ]] || continue
    url_port="${url##*:}"
    if [[ "$url_port" != "$port" ]]; then
      die "${name}_URL '${url}' does not point at the configured ${name}_PORT '${port}' -- fix lab.env or delete it and re-run lab.sh init"
    fi
  done
}

cmd_init() {
  if [[ -e "$ENV_FILE" ]]; then
    die "refusing to overwrite existing $ENV_FILE (delete it first to regenerate)"
  fi
  local rsa_key
  rsa_key="$(openssl genrsa 2048 2>/dev/null | openssl base64 -A)"
  [[ -n "$rsa_key" ]] || die "openssl failed to generate the Lago RSA private key"
  LAB_RSA_KEY_INPUT="$rsa_key" python3 "$LAB_DIR/clients.py" gen-env --output "$ENV_FILE"
}

cmd_up() {
  check_secrets
  check_port_url_consistency
  compose up -d --wait
}

cmd_down() {
  ensure_env
  compose down
}

cmd_status() {
  ensure_env
  local rows port
  port="$(env_value LAGO_API_PORT)"
  [[ -n "$port" ]] || port="48891"
  rows="$(compose ps --format json 2>/dev/null || true)"
  LAB_STATUS_ROWS="$rows" python3 - "$REPO_DIR/deploy/lago" "http://127.0.0.1:${port}" <<'PY'
import json
import os
import sys

lago_dir, api_url = sys.argv[1], sys.argv[2]
sys.path.insert(0, lago_dir)
import health  # noqa: E402  (deploy/lago/health.py, reused read-only)

rows = health._compose_rows(os.environ.get("LAB_STATUS_ROWS", ""))
api_ok = health._api_ok(f"{api_url}/health")
identity = health.load_release_identity()
print(json.dumps(health.build_snapshot(rows, api_ok, identity), sort_keys=True))
PY
}

cmd_config() {
  check_secrets
  local resolved
  resolved="$(compose config)"
  LAB_CONFIG_TEXT="$resolved" python3 - "$ENV_FILE" <<'PY'
import os
import sys
from pathlib import Path

env_file = sys.argv[1]
text = os.environ.get("LAB_CONFIG_TEXT", "")
for line in Path(env_file).read_text(encoding="utf-8").splitlines():
    line = line.strip()
    if not line or line.startswith("#") or "=" not in line:
        continue
    key, _, value = line.partition("=")
    value = value.strip().strip('"')
    if not value:
        continue
    if any(marker in key for marker in ("PASSWORD", "SECRET", "KEY", "SALT", "TOKEN")):
        text = text.replace(value, "***REDACTED***")
sys.stdout.write(text)
PY
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  case "${1:-}" in
    init) shift; cmd_init "$@" ;;
    up) shift; cmd_up "$@" ;;
    down) shift; cmd_down "$@" ;;
    status) shift; cmd_status "$@" ;;
    config) shift; cmd_config "$@" ;;
    -h|--help|help) usage ;;
    *) usage ;;
  esac
fi
