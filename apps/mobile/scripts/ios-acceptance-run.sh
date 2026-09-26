#!/usr/bin/env bash
# T39（Issue #69）：安装包验收证据管线——无凭据可验证路径（冷启动/未授权深链/未授权推送/
# 麦克风拒权/崩溃筛查）。授权面九流的自动化需要真实 deployment 凭据 + idb（本环境两者皆缺，
# 见 t39-acceptance.md 的 blocked-env 清单与人工路径清单）。
# 用法：bash apps/mobile/scripts/ios-acceptance-run.sh <booted-UDID> [仓库根]
# 产物：docs/plans/issue30-sweep/ios-evidence/t39/
set -euo pipefail

UDID="${1:?usage: ios-acceptance-run.sh <booted-UDID> [repo-root]}"
ROOT="${2:-$(pwd)}"
MOBILE="$ROOT/apps/mobile"
APP="$MOBILE/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app"
BUNDLE="com.weknora.mobile"
OUT="$ROOT/docs/plans/issue30-sweep/ios-evidence/t39"
mkdir -p "$OUT"
test -d "$APP" || { echo "Release app missing — run ios-release-build.sh first" >&2; exit 1; }

xcrun simctl bootstatus "$UDID" -b >/dev/null

# 1) 冷启动（AC2 cold-start）：干净卸载 → 首装 → 录屏启动全程 → 2s/11s 截图
xcrun simctl terminate "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl uninstall "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl io "$UDID" recordVideo --codec h264 --force "$OUT/cold-start.mov" &
REC=$!
xcrun simctl install "$UDID" "$APP"
xcrun simctl launch "$UDID" "$BUNDLE"
sleep 2
xcrun simctl io "$UDID" screenshot "$OUT/01-cold-start-2s.png" >/dev/null
sleep 9
xcrun simctl io "$UDID" screenshot "$OUT/02-login-screen-11s.png" >/dev/null
kill -INT "$REC" 2>/dev/null || true
wait "$REC" 2>/dev/null || true

# 2) 未授权深链 fail-closed 探针（AC2 撤销/越权面）：每条深链后截屏留证
probe_deep_link() {
  xcrun simctl openurl "$UDID" "$1"
  sleep 2
  xcrun simctl io "$UDID" screenshot "$OUT/$2" >/dev/null
}
probe_deep_link "weknora://tasks" "03-deeplink-tasks-unauthorized.png"
probe_deep_link "weknora://ask" "04-deeplink-ask-unauthorized.png"
probe_deep_link "weknora://tasks/detail?taskId=1&runId=1" "05-deeplink-detail-unauthorized.png"

# 3) 未授权推送投递（本地模拟 APNs 帧真机语义）：投递后进程必须仍在（崩溃由第 5 步日志门兜底）
cat > "$OUT/push-payload.json" <<'JSON'
{
  "Simulator Target Bundle": "com.weknora.mobile",
  "aps": { "alert": { "title": "WeKnora", "body": "acceptance probe" }, "sound": "default" }
}
JSON
xcrun simctl push "$UDID" "$BUNDLE" "$OUT/push-payload.json"
sleep 2
xcrun simctl io "$UDID" screenshot "$OUT/06-push-delivered-unauthorized.png" >/dev/null
xcrun simctl spawn "$UDID" launchctl list 2>/dev/null | grep "$BUNDLE" > "$OUT/push-process-alive.txt" || true

# 4) 麦克风拒权探针（AC2 permission-denied）：显式 revoke（iOS 会终止运行中的应用）→ 重启必须存活
xcrun simctl privacy "$UDID" revoke microphone "$BUNDLE"
xcrun simctl terminate "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl launch "$UDID" "$BUNDLE"
sleep 3
xcrun simctl io "$UDID" screenshot "$OUT/07-after-mic-revoked.png" >/dev/null

# 5) 启动日志崩溃筛查（B4 同款口径：fatal/未捕获/NSException/信号崩溃必须 0 命中）
xcrun simctl spawn "$UDID" log show --last 5m --predicate 'processImagePath CONTAINS "WeKnora"' > "$OUT/app-launch-log.txt" 2>&1 || true
ERRS="$(grep -cE 'fatal|uncaught|NSException|SIGTRAP|SIGSEGV' "$OUT/app-launch-log.txt" || true)"
echo "FATAL_LOG_HITS=$ERRS"
if [ "$ERRS" -ne 0 ]; then echo "crash signals found in the launch log — acceptance probes FAILED" >&2; exit 1; fi
echo "ACCEPTANCE_PROBES_OK=$OUT"
