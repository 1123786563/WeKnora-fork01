#!/usr/bin/env bash
# W06 craft browser-acceptance stack harness.
#
# Boots the REAL full stack for e2e/craft-report.spec.ts, runs Playwright,
# then tears down exactly the processes it started (recorded PIDs / the
# temp nginx instance only — nothing broad, nothing shared).
#
# Components (all bound to 127.0.0.1 on fixed high ports):
#   main-model mock (node)     — the craft main agent decides to delegate
#   sub-model mock (node)      — the OpenCode provider for mock mode
#   opencode serve (pinned)    — XDG-isolated; mock or real free model
#   Go server (cmd/server)     — SQLite + local storage + craft enabled
#   vite dev server            — apps/web with VITE_API_BASE_URL
#   nginx preview origin       — deploy/craft/preview.conf + self-signed cert
#
# Usage:
#   craft-stack.sh up   [mock|real]   boot the stack (returns when ready;
#                                    run dir recorded in /tmp/craft-w06-<mode>.latest)
#   craft-stack.sh run   [mock|real] [-- extra playwright args]
#                                    run the acceptance spec against the live stack
#   craft-stack.sh down  [mock|real]   precise teardown of this stack only
#   craft-stack.sh mock|real          up -> run -> down in one shot
#
# Every process PID is recorded; teardown never matches by name and never
# touches anything outside this harness's own run directory.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
ACTION="${1:-}"
if [ "$ACTION" = "mock" ] || [ "$ACTION" = "real" ]; then
  MODE="$ACTION"; ACTION="all"
else
  MODE="${2:-mock}"
fi
if [ "$MODE" != "mock" ] && [ "$MODE" != "real" ]; then
  echo "usage: $0 [up|run|down|all] [mock|real]" >&2; exit 2
fi

# The stack tag namespaces the run-dir pointer so parallel task worktrees
# never cross-teardown each other (ports above must differ too).
STACK_TAG="${CRAFT_STACK_TAG:-w06}"
LATEST="/tmp/craft-$STACK_TAG-$MODE.latest"
if [ "$ACTION" = "up" ] || [ "$ACTION" = "all" ]; then
  RUN="$(mktemp -d "/tmp/craft-$STACK_TAG-$MODE.XXXXXX")"
else
  [ -f "$LATEST" ] || { echo "no stack running for mode $MODE" >&2; exit 2; }
  RUN="$(cat "$LATEST")"
fi
ART="$RUN/artifacts"; mkdir -p "$ART" "$RUN/bin" "$RUN/certs" "$RUN/logs"
PIDS="$RUN/pids.txt"; [ -f "$PIDS" ] || : > "$PIDS"

# Fixed high ports by default; every one is env-overridable so parallel
# task worktrees can stagger their stacks (C03-era single-consumer rule).
PORT_MAIN="${CRAFT_PORT_MAIN:-41871}"; PORT_SUB="${CRAFT_PORT_SUB:-41872}"
PORT_OC="${CRAFT_PORT_OC:-41873}"; PORT_OC_REAL="${CRAFT_PORT_OC_REAL:-41883}"
PORT_API="${CRAFT_PORT_API:-41875}"; PORT_VITE="${CRAFT_PORT_VITE:-41876}"
PORT_PREVIEW="${CRAFT_PORT_PREVIEW:-41877}"; PORT_PLAIN="${CRAFT_PORT_PLAIN:-41878}"
WEB_ORIGIN="http://127.0.0.1:$PORT_VITE"
API_ORIGIN="http://127.0.0.1:$PORT_API"
PREVIEW_ORIGIN="https://127.0.0.1:$PORT_PREVIEW"

OC_BIN=/Users/wuyongjun/.opencode/bin/opencode
OC_SHA256=9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98

cleanup() {
  local status=$?
  # A completed `up` leaves the stack running on purpose; a FAILED up must
  # still clean up after itself.
  [ -f "$RUN/UP_OK" ] && return 0
  echo "[stack] teardown (exit $status)"
  rm -f "$LATEST"
  if [ -f "$RUN/nginx.pid" ]; then
    kill "$(cat "$RUN/nginx.pid")" 2>/dev/null || true
  fi
  if [ -f "$RUN/preview.conf" ]; then /opt/homebrew/bin/nginx -c "$RUN/preview.conf" -s stop >/dev/null 2>&1 || true; fi
  while read -r pid name; do
    [ -n "${pid:-}" ] || continue
    if kill -0 "$pid" 2>/dev/null; then
      echo "[stack] stopping $name (pid $pid, process group)"
      # Every detached component is a session/process-group leader
      # (start_new_session=True), so -PGID reaps its whole subtree — a
      # plain pid kill left the vite wrapper's node children alive.
      kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    fi
  done < "$PIDS"
  sleep 1
  while read -r pid name; do
    [ -n "${pid:-}" ] || continue
    kill -KILL -- "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
  done < "$PIDS"
}
trap cleanup EXIT

note_pid() { echo "$1 $2" >> "$PIDS"; }  # line format: <pid> <name>

stack_down() {
  echo "[stack] teardown"
  rm -f "$LATEST"
  # nginx daemonizes: stop it by its recorded pid file / conf, not by name.
  if [ -f "$RUN/nginx.pid" ]; then
    kill "$(cat "$RUN/nginx.pid")" 2>/dev/null || true
  fi
  if [ -f "$RUN/preview.conf" ]; then /opt/homebrew/bin/nginx -c "$RUN/preview.conf" -s stop >/dev/null 2>&1 || true; fi
  while read -r pid name; do
    [ -n "${pid:-}" ] || continue
    if kill -0 "$pid" 2>/dev/null; then
      echo "[stack] stopping $name (pid $pid, process group)"
      # Every detached component is a session/process-group leader
      # (start_new_session=True), so -PGID reaps its whole subtree — a
      # plain pid kill left the vite wrapper's node children alive.
      kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    fi
  done < "$PIDS"
  sleep 1
  while read -r pid name; do
    [ -n "${pid:-}" ] || continue
    kill -KILL -- "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
  done < "$PIDS"
}

# detach <logfile> <cwd> <command...>: start a long-running component in a
# NEW SESSION so it survives this bash invocation (the command runner reaps
# its own process tree when a command finishes). Prints the child pid.
detach() {
  local log="$1" cwd="$2"; shift 2
  python3 - "$log" "$cwd" "$@" <<'PY'
import subprocess, sys
log = open(sys.argv[1], 'ab', buffering=0)
child = subprocess.Popen(sys.argv[3:], cwd=sys.argv[2], stdout=log, stderr=log,
                        stdin=subprocess.DEVNULL, start_new_session=True)
print(child.pid)
PY
}

if [ "$ACTION" = "down" ]; then
  stack_down
  echo "[stack] down; run dir kept for inspection: $RUN"
  trap - EXIT
  exit 0
fi

if [ "$ACTION" = "run" ]; then
  # Playwright against the already-running stack. Extra args after the mode
  # reach playwright verbatim (e.g. `run mock --grep 01`).
  if [ $# -ge 2 ]; then shift 2; fi
  SPEC_ARGS=("e2e/craft-report.spec.ts")
  # An optional spec path (first non-flag arg) selects which craft spec runs
  # (default: the W06 report acceptance); anything after it, or after --,
  # reaches playwright verbatim.
  if [ "$#" -gt 0 ] && [ "${1#-}" = "$1" ]; then SPEC_ARGS=("$1"); shift; fi
  set +e
  (cd "$ROOT/apps/web" && env \
    CRAFT_WEB_URL="$WEB_ORIGIN" \
    CRAFT_AUTH_STATE="$RUN/auth-owner.json" \
    CRAFT_VIEWER_STATE="$RUN/auth-viewer.json" \
    CRAFT_FOREIGN_STATE="$RUN/auth-foreign.json" \
    CRAFT_API_URL="$API_ORIGIN/api/v1" \
    CRAFT_DB_PATH="$RUN/weknora.db" \
    CRAFT_MODEL_MODE="$MODE" \
    CRAFT_PROBE_APP_ORIGIN="$WEB_ORIGIN" \
    CRAFT_KINDS="${CRAFT_KINDS:-web}" \
    CRAFT_E2E_OUTPUT="$ART" \
    pnpm exec playwright test -c playwright.craft.config.ts "${SPEC_ARGS[@]}" "$@") 2>&1 | tee "$RUN/logs/playwright.log"
  status=${PIPESTATUS[0]}
  set -e
  echo "[stack] playwright exit=$status artifacts=$ART run=$RUN"
  trap - EXIT
  exit "$status"
fi

wait_http() { # url label [timeout_seconds]
  local url="$1" label="$2" budget="${3:-90}" elapsed=0
  while [ "$elapsed" -lt "$budget" ]; do
    if curl -sk -o /dev/null -m 3 "$url"; then echo "[stack] $label ready (${elapsed}s)"; return 0; fi
    sleep 1; elapsed=$((elapsed + 1))
  done
  echo "[stack] $label NEVER became ready ($url)" >&2; return 1
}

echo "[stack] mode=$MODE run=$RUN"
echo "[stack] repository=$ROOT"

# --- 1. Build the server binary (locked toolchain, repo modules) ----------
echo "[stack] building cmd/server ..."
(cd "$ROOT" && go build -o "$RUN/bin/weknora-server" ./cmd/server) >"$RUN/logs/build.log" 2>&1

# --- 2. Model fixtures -----------------------------------------------------
note_pid "$(detach "$RUN/logs/main-model.log" "$ROOT" node "$ROOT/apps/web/e2e/craft-mocks/main-model.mjs" "$PORT_MAIN")" main-model
wait_http "http://127.0.0.1:$PORT_MAIN/v1/models" "main-model fixture" 30

if [ "$MODE" = "mock" ]; then
  note_pid "$(CRAFT_PROBE_APP_ORIGIN="$WEB_ORIGIN" detach "$RUN/logs/sub-model.log" "$ROOT" node "$ROOT/apps/web/e2e/craft-mocks/sub-model.mjs" "$PORT_SUB")" sub-model
  wait_http "http://127.0.0.1:$PORT_SUB/v1/models" "sub-model fixture" 30
fi

# --- 3. Pinned OpenCode serve (XDG-isolated, credentials scrubbed) --------
ACTUAL_SHA="$(shasum -a 256 "$OC_BIN" | cut -d' ' -f1)"
if [ "$ACTUAL_SHA" != "$OC_SHA256" ]; then
  echo "[stack] opencode binary digest changed: $ACTUAL_SHA (want $OC_SHA256)" >&2; exit 1
fi
"$OC_BIN" --version | grep -q '1.18.4' || { echo '[stack] opencode version mismatch' >&2; exit 1; }

SERVE_DIR="$RUN/serve-$MODE"; mkdir -p "$SERVE_DIR"
XDG="$RUN/xdg-$MODE"
if [ "$MODE" = "mock" ]; then
  mkdir -p "$XDG/config/opencode"
  cat > "$XDG/config/opencode/opencode.json" <<'OCJSON'
{
  "$schema": "https://opencode.ai/config.json",
  "model": "mock/mock-model",
  "provider": {
    "mock": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Local Mock",
      "options": {"baseURL": "http://127.0.0.1:PORT_SUB_PLACEHOLDER/v1", "apiKey": "local-mock-not-a-real-key"},
      "models": {"mock-model": {"name": "Mock Model"}}
    }
  },
  "permission": {"edit": "allow", "bash": {"*": "allow"}}
}
OCJSON
  sed -i '' "s|PORT_SUB_PLACEHOLDER|$PORT_SUB|" "$XDG/config/opencode/opencode.json"
  OC_PORT="$PORT_OC"
else
  # Real mode: no provider config at all — the serve falls back to its
  # built-in free model. The XDG triple stays isolated and the environment
  # below carries no credential-shaped variables, so no user credential is
  # loaded or consumed (R07 scheme).
  OC_PORT="$PORT_OC_REAL"
fi

OC_ENV="PATH=$PATH HOME=$RUN/oc-home-$MODE TMPDIR=$TMPDIR XDG_CONFIG_HOME=$XDG/config XDG_DATA_HOME=$XDG/data XDG_STATE_HOME=$XDG/state"
note_pid "$(OC_ENV="$OC_ENV" detach "$RUN/logs/oc-serve.log" "$SERVE_DIR" env $OC_ENV "$OC_BIN" serve --port "$OC_PORT" --hostname 127.0.0.1)" opencode-serve
wait_http "http://127.0.0.1:$OC_PORT/doc" "opencode serve ($MODE)" 120

# --- 4. Server config tree (config.yaml + prompt templates) ---------------
mkdir -p "$RUN/config"
cp -R "$ROOT/config/." "$RUN/config/"
# The migration source path is cwd-relative (file://migrations/sqlite).
ln -sF "$ROOT/migrations" "$RUN/migrations"
sed -i '' -e "s/^  port: .*/  port: $PORT_API/" -e "s/^  host: .*/  host: \"127.0.0.1\"/" "$RUN/config/config.yaml"

# --- 5. Go server ----------------------------------------------------------
JWT_SECRET="$(openssl rand -hex 24)"
DB_PASSWORD_UNUSED=x
STORAGE_PATH="$RUN/storage"; mkdir -p "$STORAGE_PATH"
note_pid "$(detach "$RUN/logs/server.log" "$RUN" env \
  GIN_MODE=release \
  DB_DRIVER=sqlite DB_PATH="$RUN/weknora.db" \
  STORAGE_TYPE=local LOCAL_STORAGE_BASE_DIR="$STORAGE_PATH" \
  SSRF_WHITELIST="127.0.0.1,localhost" \
  JWT_SECRET="$JWT_SECRET" \
  WEKNORA_CRAFT_ENABLED=true \
  WEKNORA_CRAFT_KINDS="${CRAFT_KINDS:-web}" \
  WEKNORA_CRAFT_APP_ORIGIN="$WEB_ORIGIN" \
  WEKNORA_CRAFT_PREVIEW_ORIGIN="$PREVIEW_ORIGIN" \
  CRAFT_OPENCODE_BASE_URL="http://127.0.0.1:$OC_PORT" \
  CRAFT_OPENCODE_WORK_DIR="$SERVE_DIR" \
  WEKNORA_AGENT_RECOVERY_ENABLED=true \
  WEKNORA_AGENT_RECOVERY_ADMISSION_ENABLED=true \
  "$RUN/bin/weknora-server")" weknora-server
wait_http "$API_ORIGIN/health" "go server" 120

# --- 6. Seed test identities + the main model (random credentials) --------
PASS_OWNER="$(openssl rand -hex 12)"; PASS_VIEWER="$(openssl rand -hex 12)"; PASS_FOREIGN="$(openssl rand -hex 12)"
HASHGEN="$RUN/hashgen"; mkdir -p "$HASHGEN"
cat > "$HASHGEN/main.go" <<'GOEOF'
package main

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for _, pw := range os.Args[1:] {
		hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err != nil { panic(err) }
		fmt.Fprintln(out, string(hash))
	}
}
GOEOF
(cd "$HASHGEN" && go mod init hashgen >/dev/null 2>&1 && go mod edit -require golang.org/x/crypto@v0.53.0 && go mod tidy >/dev/null 2>&1 && go build -o hashgen .) >>"$RUN/logs/build.log" 2>&1
printf 'owner=%s\nviewer=%s\nforeign=%s\n' "$PASS_OWNER" "$PASS_VIEWER" "$PASS_FOREIGN" > "$RUN/creds.txt"
chmod 600 "$RUN/creds.txt"
H_OWNER="$($HASHGEN/hashgen "$PASS_OWNER")"
H_VIEWER="$($HASHGEN/hashgen "$PASS_VIEWER")"
H_FOREIGN="$($HASHGEN/hashgen "$PASS_FOREIGN")"

sqlite3 "$RUN/weknora.db" <<SQLEOF
.timeout 10000
INSERT INTO tenants (id, name, business, status) VALUES (1, 'craft-w06-t1', 'w06 acceptance', 'active');
INSERT INTO tenants (id, name, business, status) VALUES (2, 'craft-w06-t2', 'w06 acceptance', 'active');
INSERT INTO users (id, username, email, password_hash, tenant_id, is_active) VALUES ('w06-owner', 'w06-owner', 'owner@w06.test', '$H_OWNER', 1, 1);
INSERT INTO users (id, username, email, password_hash, tenant_id, is_active) VALUES ('w06-viewer', 'w06-viewer', 'viewer@w06.test', '$H_VIEWER', 1, 1);
INSERT INTO users (id, username, email, password_hash, tenant_id, is_active) VALUES ('w06-foreign', 'w06-foreign', 'foreign@w06.test', '$H_FOREIGN', 2, 1);
INSERT INTO tenant_members (user_id, tenant_id, role, status, joined_at) VALUES ('w06-owner', 1, 'owner', 'active', CURRENT_TIMESTAMP);
INSERT INTO tenant_members (user_id, tenant_id, role, status, joined_at) VALUES ('w06-viewer', 1, 'viewer', 'active', CURRENT_TIMESTAMP);
INSERT INTO tenant_members (user_id, tenant_id, role, status, joined_at) VALUES ('w06-foreign', 2, 'owner', 'active', CURRENT_TIMESTAMP);
INSERT INTO models (id, tenant_id, name, display_name, type, source, parameters, is_default, is_builtin, status) VALUES ('craft-main-fixture', 1, 'craft-main-fixture', 'Craft Main Fixture', 'VLLM', 'remote', '{"base_url":"http://127.0.0.1:$PORT_MAIN/v1","api_key":"local-main-fixture","provider":"openai","interface_type":"chat"}', 1, 0, 'active');
-- config/builtin_models.yaml syncs builtin-llm-mock (a LAN mock endpoint,
-- unreachable here) as is_default=true BEFORE this seed runs; craftChatModelID
-- returns the first default it meets, so the builtin row would win and the
-- run would die on the SSRF/unreachable base_url. Demote it explicitly.
UPDATE models SET is_default = 0 WHERE id = 'builtin-llm-mock';
SQLEOF
echo "[stack] seeded identities (credentials only in this run dir)"

# --- 7. Real logins -> storageState files ----------------------------------
login() { curl -s -m 20 -X POST "$API_ORIGIN/api/v1/auth/login" -H 'content-type: application/json' -d "{\"email\":\"$1\",\"password\":\"$2\"}"; }
login_status() { curl -s -m 20 -o /dev/null -w '%{http_code}' -X POST "$API_ORIGIN/api/v1/auth/login" -H 'content-type: application/json' -d "{\"email\":\"$1\",\"password\":\"$2\"}"; }
TOKEN_OWNER="$(login owner@w06.test "$PASS_OWNER" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const j=JSON.parse(s);process.stdout.write(j.token||"")})')"
TOKEN_VIEWER="$(login viewer@w06.test "$PASS_VIEWER" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const j=JSON.parse(s);process.stdout.write(j.token||"")})')"
TOKEN_FOREIGN="$(login foreign@w06.test "$PASS_FOREIGN" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const j=JSON.parse(s);process.stdout.write(j.token||"")})')"
for t in OWNER VIEWER FOREIGN; do
  eval "tok=\$TOKEN_$t"
  if [ -z "$tok" ]; then
    eval "email_var=EMAIL_$t"
    echo "[stack] login failed for $t (status $(login_status "owner@w06.test" "bad-probe" >/dev/null; echo see-log))" >&2
    tail -3 "$RUN/logs/server.log" >&2 || true
    exit 1
  fi
done
node -e '
const fs = require("fs");
const web = process.argv[1]; const dir = process.argv[2];
const tokens = { owner: process.argv[3], viewer: process.argv[4], foreign: process.argv[5] };
const tenants = { owner: "1", viewer: "1", foreign: "2" };
for (const [name, token] of Object.entries(tokens)) {
  fs.writeFileSync(`${dir}/auth-${name}.json`, JSON.stringify({ cookies: [], origins: [{ origin: web, localStorage: [
    { name: "weknora_token", value: token },
    { name: "weknora_selected_tenant_id", value: tenants[name] },
  ]}]}, null, 2));
}
' "$WEB_ORIGIN" "$RUN" "$TOKEN_OWNER" "$TOKEN_VIEWER" "$TOKEN_FOREIGN"
echo "[stack] storageState files written"

# --- 8. Preview origin: W02 preview.conf + self-signed cert ----------------
openssl req -x509 -newkey rsa:2048 -nodes -days 4 \
  -keyout "$RUN/certs/privkey.pem" -out "$RUN/certs/fullchain.pem" \
  -subj "/CN=127.0.0.1" -addext "subjectAltName=IP:127.0.0.1,DNS:localhost" >/dev/null 2>&1
sed -e "s|listen 443 ssl;|listen $PORT_PREVIEW ssl;|" \
    -e "s|server_name PREVIEW_HOSTNAME;|server_name _;|" \
    -e "s|/etc/nginx/certs/preview/fullchain.pem|$RUN/certs/fullchain.pem|" \
    -e "s|/etc/nginx/certs/preview/privkey.pem|$RUN/certs/privkey.pem|" \
    -e "s|listen 80;|listen $PORT_PLAIN;|" \
    -e "s|http://APP_UPSTREAM|http://127.0.0.1:$PORT_API|" \
    -e "s|server_name PREVIEW_HOSTNAME;|server_name _;|" \
    "$ROOT/deploy/craft/preview.conf" > "$RUN/preview.conf"
cat > "$RUN/preview-head.conf" <<NGINXHEAD
pid $RUN/nginx.pid;
error_log $RUN/logs/nginx-error.log;
NGINXHEAD
cat "$RUN/preview-head.conf" "$RUN/preview.conf" > "$RUN/preview.conf.tmp" && mv "$RUN/preview.conf.tmp" "$RUN/preview.conf"
sed -i '' "s|access_log off;|access_log off;\n    client_body_temp_path $RUN/nginx-client;\n    proxy_temp_path $RUN/nginx-proxy;\n    fastcgi_temp_path $RUN/nginx-fastcgi;\n    uwsgi_temp_path $RUN/nginx-uwsgi;\n    scgi_temp_path $RUN/nginx-scgi;|" "$RUN/preview.conf"
/opt/homebrew/bin/nginx -t -c "$RUN/preview.conf" >/dev/null 2>&1 || { echo '[stack] nginx -t failed' >&2; sed -n 1,40p "$RUN/preview.conf" >&2; exit 1; }
/opt/homebrew/bin/nginx -c "$RUN/preview.conf"
wait_http "$PREVIEW_ORIGIN/" "nginx preview origin (any status counts as listening)" 30 || true

# --- 9. Vite dev server ----------------------------------------------------
note_pid "$(detach "$RUN/logs/vite.log" "$ROOT/apps/web" env VITE_API_BASE_URL="$API_ORIGIN" \
  pnpm exec vite --port "$PORT_VITE" --strictPort --host 127.0.0.1)" vite
wait_http "$WEB_ORIGIN/" "vite dev server" 90

# --- 10. Up phase complete ---------------------------------------------------
echo "$RUN" > "$LATEST"
cat > "$RUN/stack-env.sh" <<ENVOUT
export CRAFT_WEB_URL="$WEB_ORIGIN"
export CRAFT_AUTH_STATE="$RUN/auth-owner.json"
export CRAFT_VIEWER_STATE="$RUN/auth-viewer.json"
export CRAFT_FOREIGN_STATE="$RUN/auth-foreign.json"
export CRAFT_API_URL="$API_ORIGIN/api/v1"
export CRAFT_DB_PATH="$RUN/weknora.db"
export CRAFT_MODEL_MODE="$MODE"
export CRAFT_PROBE_APP_ORIGIN="$WEB_ORIGIN"
export CRAFT_KINDS="${CRAFT_KINDS:-web}"
export CRAFT_E2E_OUTPUT="$ART"
ENVOUT
echo "[stack] up complete: run=$RUN"
echo "[stack]   web=$WEB_ORIGIN api=$API_ORIGIN preview=$PREVIEW_ORIGIN"
echo "[stack]   next: $0 run $MODE   (teardown: $0 down $MODE)"

if [ "$ACTION" = "up" ]; then
  touch "$RUN/UP_OK"
  trap - EXIT
  exit 0
fi

# --- 11. All-in-one: run the acceptance, then tear down -----------------------
echo "[stack] running playwright ($MODE) ..."
set +e
bash "$0" run "$MODE"
PW_STATUS=$?
set -e
echo "[stack] playwright exit=$PW_STATUS"
echo "[stack] artifacts: $ART"
echo "[stack] run dir (logs/db/storage): $RUN"
if [ "$PW_STATUS" -eq 0 ]; then touch "$RUN/PASSED"; fi
stack_down
trap - EXIT
exit "$PW_STATUS"