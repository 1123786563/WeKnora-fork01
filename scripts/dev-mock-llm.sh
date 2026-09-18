#!/usr/bin/env bash
# R476 A1: daemon controller for the local mock LLM (scripts/dev-mock-llm.py).
# Replaces R475's session-scoped manual restart with a persistent process:
#   nohup python3 scripts/dev-mock-llm.py >> /tmp/weknora-dev-mock-llm.log 2>&1 &
#   pid -> /tmp/weknora-dev-mock-llm.pid
# No credentials involved; the mock only serves deterministic echoes on
# 0.0.0.0:18090 (LAN reach is gated by the backend SSRF whitelist in .env.local).
#
# Usage:  bash scripts/dev-mock-llm.sh {start|stop|restart|status}
# npm:    pnpm dev:mock-llm | pnpm dev:mock-llm:stop | pnpm dev:mock-llm:status
set -euo pipefail

PORT=18090
PIDFILE=/tmp/weknora-dev-mock-llm.pid
LOGFILE=/tmp/weknora-dev-mock-llm.log
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PY_SCRIPT="$SCRIPT_DIR/dev-mock-llm.py"

pid_alive() {
  [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null
}

port_owner() {
  lsof -nP -t -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | head -1 || true
}

cmd_start() {
  if pid_alive; then
    echo "[mock-llm] already running (pid $(cat "$PIDFILE")) on :$PORT; log: $LOGFILE"
    return 0
  fi
  local owner
  owner="$(port_owner)"
  if [ -n "$owner" ]; then
    if ps -p "$owner" -o command= 2>/dev/null | grep -q "mock-llm.py"; then
      echo "[mock-llm] adopting already-running mock (pid $owner, not from this controller)" >&2
      echo "$owner" > "$PIDFILE"
      return 0
    fi
    echo "[mock-llm] ERROR: port :$PORT held by foreign pid $owner ($(ps -p "$owner" -o comm= 2>/dev/null)); refusing to start." >&2
    return 1
  fi
  nohup python3 "$PY_SCRIPT" >>"$LOGFILE" 2>&1 &
  local pid=$!
  echo "$pid" > "$PIDFILE"
  local i
  for i in $(seq 1 40); do
    if curl -sf -o /dev/null "http://127.0.0.1:$PORT/v1/models"; then
      echo "[mock-llm] started (pid $pid) -> http://127.0.0.1:$PORT/v1 (LAN: http://192.168.3.30:$PORT/v1)"
      echo "[mock-llm] pidfile: $PIDFILE  log: $LOGFILE"
      return 0
    fi
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.25
  done
  echo "[mock-llm] ERROR: not ready after 10s; tail of $LOGFILE:" >&2
  tail -5 "$LOGFILE" >&2 || true
  rm -f "$PIDFILE"
  return 1
}

kill_pid() {
  local pid="$1"
  kill "$pid" 2>/dev/null || true
  local i
  for i in $(seq 1 30); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 0.1
  done
  kill -9 "$pid" 2>/dev/null || true
}

cmd_stop() {
  if pid_alive; then
    local pid
    pid="$(cat "$PIDFILE")"
    kill_pid "$pid"
    rm -f "$PIDFILE"
    echo "[mock-llm] stopped (pid $pid)"
    return 0
  fi
  rm -f "$PIDFILE"
  local owner
  owner="$(port_owner)"
  if [ -n "$owner" ] && ps -p "$owner" -o command= 2>/dev/null | grep -q "mock-llm.py"; then
    kill_pid "$owner"
    echo "[mock-llm] stopped stray mock instance (pid $owner)"
    return 0
  fi
  echo "[mock-llm] not running"
  return 0
}

cmd_status() {
  if pid_alive; then
    echo "running (pid $(cat "$PIDFILE")) on :$PORT"
    return 0
  fi
  rm -f "$PIDFILE"
  local owner
  owner="$(port_owner)"
  if [ -n "$owner" ]; then
    if ps -p "$owner" -o command= 2>/dev/null | grep -q "mock-llm.py"; then
      echo "running (unmanaged mock, pid $owner) on :$PORT — not in $PIDFILE; 'start' will adopt it, 'stop' will kill it"
      return 0
    fi
    echo "stopped, but :$PORT is held by foreign pid $owner"
    return 3
  fi
  echo "stopped"
  return 3
}

case "${1:-start}" in
  start)   cmd_start ;;
  stop)    cmd_stop ;;
  restart) cmd_stop; cmd_start ;;
  status)  cmd_status ;;
  *) echo "usage: $0 {start|stop|restart|status}" >&2; exit 2 ;;
esac
