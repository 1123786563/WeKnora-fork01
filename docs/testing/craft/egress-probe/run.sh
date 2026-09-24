#!/usr/bin/env bash
set -euo pipefail
IMAGE="${IMAGE:-weknora-craft-runtime:1.18.4}"
NET="craft-egress-probe-$$"
ADAPTER="craft-egress-adapter-$$"
HOME_DIR="$(mktemp -d)"
RAW="$(pwd)/docs/testing/craft/egress-probe/raw-output-$(date +%Y%m%d%H%M%S).txt"
cleanup(){ docker rm -f "$ADAPTER" craft-egress-direct-$$ craft-egress-run-$$ >/dev/null 2>&1 || true; docker network rm "$NET" >/dev/null 2>&1 || true; rm -rf "$HOME_DIR"; }
trap cleanup EXIT
docker image inspect "$IMAGE" --format 'image_id={{.Id}} arch={{.Architecture}} os={{.Os}}' | tee "$RAW"
docker run --rm --entrypoint sh "$IMAGE" -lc 'opencode --version' | tee -a "$RAW"
BASE="craft-egress-base-$$"; docker network create "$BASE" >/dev/null
docker run -d --name "craft-egress-direct-$$" --network "$BASE" --network-alias direct-listener -v "$(pwd)/docs/testing/craft/egress-probe:/probe:ro" python:3.12-alpine python /probe/mock_server.py >/dev/null
sleep 1
docker run --rm --entrypoint sh --network "$BASE" "$IMAGE" -lc 'curl -sS --max-time 3 -o /dev/null -w "baseline_direct_http=%{http_code}\n" http://direct-listener:8080/v1/models' 2>&1 | tee -a "$RAW" || true
docker rm -f "craft-egress-direct-$$" >/dev/null; docker network rm "$BASE" >/dev/null
docker network create --internal "$NET" >/dev/null
docker run -d --name "$ADAPTER" --network "$NET" --network-alias mock-adapter -e LOG=/tmp/mock-log.jsonl -v "$(pwd)/docs/testing/craft/egress-probe:/probe:ro" python:3.12-alpine python /probe/mock_server.py >/dev/null
docker run -d --name "craft-egress-direct-$$" --network "$NET" --network-alias direct-listener -v "$(pwd)/docs/testing/craft/egress-probe:/probe:ro" python:3.12-alpine python /probe/mock_server.py >/dev/null
sleep 1
docker run --rm --entrypoint sh --name craft-egress-run-$$ --network "$NET" -e HOME=/tmp/home -v "$HOME_DIR:/tmp/home" -v "$(pwd)/docs/testing/craft/egress-probe/opencode.json:/workspace/opencode.json:ro" "$IMAGE" -lc "cd /workspace && opencode --print-logs --model mock/mock-model run 'Reply with one word.'" 2>&1 | tee -a "$RAW" || true
docker run --rm --entrypoint sh --name craft-egress-run-$$ --network "$NET" -e HOME=/tmp/home -v "$HOME_DIR:/tmp/home" -v "$(pwd)/docs/testing/craft/egress-probe/opencode.json:/workspace/opencode.json:ro" "$IMAGE" -lc "cd /workspace && opencode --print-logs --model mock/mock-model run 'Reply after restart.'" 2>&1 | tee -a "$RAW" || true
echo '--- adapter log ---'
docker exec "$ADAPTER" sh -c 'cat /tmp/mock-log.jsonl 2>/dev/null || true' | tee -a "$RAW"
echo '--- network inspect ---'
docker network inspect "$NET" | tee -a "$RAW"
echo '--- direct IP/DNS denial ---'
docker run --rm --entrypoint sh --network "$NET" "$IMAGE" -lc 'set +e; curl -sS --max-time 2 -o /dev/null -w "direct_ip_rc=%{exitcode} http=%{http_code}\n" http://1.1.1.1/; curl -sS --max-time 2 -o /dev/null -w "provider_dns_rc=%{exitcode} http=%{http_code}\n" https://api.openai.com/v1/models' | tee -a "$RAW"
echo '--- image ---'
docker image inspect "$IMAGE" --format 'ID={{.Id}} Arch={{.Architecture}} OS={{.Os}}' | tee -a "$RAW"
echo "raw_output=$RAW"
