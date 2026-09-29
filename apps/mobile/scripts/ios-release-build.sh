#!/usr/bin/env bash
# T39（Issue #69）：可复现的 iOS 模拟器 Release 构建管线（幂等，可反复重跑）。
# 前置：macOS + Xcode 27、根目录已 pnpm install、CocoaPods 可用。
# 用法：bash apps/mobile/scripts/ios-release-build.sh [仓库根，默认当前目录]
# 产物：apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app
#       docs/plans/issue30-sweep/ios-evidence/t39/xcodebuild-release.log
#
# ⚠️ ios/ 工程永不入库（根 .gitignore 的 `apps/mobile/ios/`；8d39806f4 曾提交、80fc3648a
#    已移除，`git ls-files apps/mobile/ios` 恒为 0）——iOS 构建唯一受支持的入口就是本脚本
#    （clean prebuild + pod install），后续批次任务书不得再写「ios 工程已生成并提交」。
#    直接对本地遗留 ios/ 跑裸 xcodebuild 会静默产出缺 expo-audio/expo-network 原生模块的
#    包（B5 复验 important 发现，2026-09-28）；第 3.5 步的守卫会把这种漂移点名成硬失败。
set -euo pipefail

ROOT="${1:-$(pwd)}"
MOBILE="$ROOT/apps/mobile"
IOS="$MOBILE/ios"
EVIDENCE_DIR="${IOS_BUILD_EVIDENCE_DIR:-$ROOT/docs/plans/issue30-sweep/ios-evidence/t39}"
mkdir -p "$EVIDENCE_DIR"
cd "$MOBILE"

# 0) watchman 预热（B5 复验 minor 发现）：Metro export:embed 走 watchman，而 watch 根会
#    归并到主仓库根（.worktrees/ 23G、26 个 worktree），首次 crawl 实测 ~15 分钟，全部
#    落在 xcodebuild 计时内。这里提前触发 crawl，让它与 prebuild/pod install 并行；
#    best-effort，无 watchman 或失败都不阻塞。验收机可再提前手动 `watchman watch-project <主仓库根>` 预热。
if command -v watchman >/dev/null 2>&1; then
  watchman watch-project "$ROOT" >/dev/null 2>&1 || true &
fi

# 1) clean prebuild 丢弃任何忽略的旧 SDK 输出并重新生成 ios/（.gitignore 不入库）。
npx expo prebuild -p ios --clean --no-install

# 1.5) 验证当前 Expo SDK 生成的 scene/delegate/link/deployment contract，再安装 Pods。
pnpm exec tsx scripts/verify-ios-scene-project.ts "$IOS"

# 2) pnpm 工作区下 babel-preset-expo 的符号链接在干净安装后缺失
#    （B3 实测：Metro「Cannot find module 'babel-preset-expo'」失败于 RN bundle 阶段）——缺则重建。
if [ ! -e "$MOBILE/node_modules/babel-preset-expo" ]; then
  preset="$(ls -d "$MOBILE/node_modules/.pnpm"/babel-preset-expo@*/node_modules/babel-preset-expo 2>/dev/null | head -1)"
  if [ -z "$preset" ]; then echo "babel-preset-expo not found in the pnpm store; run pnpm install first" >&2; exit 1; fi
  ln -s "$preset" "$MOBILE/node_modules/babel-preset-expo"
fi

# 3) Pods
cd "$IOS"
pod install

# 3.5) 原生依赖漂移守卫（B5 复验 important 发现的固化）：旧 Podfile.lock 缺新 expo 模块时
#      xcodebuild 依旧 SUCCEEDED 但产物缺原生模块、仅运行时 require 才暴露（B5 实证）。
#      以 expo-modules-autolinking 的解析结果（node_modules 现状）逐一对照 Podfile.lock 的
#      PODS 节，缺口即硬失败——把 20 分钟的静默错包挡在 xcodebuild 之前。逻辑与单测在
#      src/ios-native-deps.ts（7 用例，布局同 mimosa 裁决 3）。
cd "$MOBILE"
pnpm exec tsx scripts/verify-ios-native-deps.ts

# 4) 模拟器 Release 全量构建。注意：build/ 已存在时无需删除；确需删除则必须回到
#    第 3 步重建（codegen 生成源码在 build/generated/ios/ReactCodegen 下，B4 实测）。
cd "$IOS"
xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -sdk iphonesimulator \
  -configuration Release -derivedDataPath build \
  -destination 'generic/platform=iOS Simulator' build 2>&1 | tee "$EVIDENCE_DIR/xcodebuild-release.log"

APP="$IOS/build/Build/Products/Release-iphonesimulator/WeKnora.app"
test -d "$APP" || { echo "expected Release app missing: $APP" >&2; exit 1; }
# 5) Verify every non-system @rpath framework load from an embedded framework resolves
#    inside this app. Use otool on the actual built binaries; do not repair packaging by copying Pods.
FRAMEWORKS="$APP/Frameworks"
test -d "$FRAMEWORKS" || { echo "expected embedded Frameworks directory missing: $FRAMEWORKS" >&2; exit 1; }
if grep -q '"ios.buildReactNativeFromSource": "true"' "$IOS/Podfile.properties.json" \
  && [ -d "$FRAMEWORKS/ExpoModulesWorklets.framework" ]; then
  echo "FRAMEWORK_MODE_MISMATCH: source-built React Native must not embed precompiled ExpoModulesWorklets.framework" >&2
  exit 1
fi
FRAMEWORKS_DIR="$FRAMEWORKS" python3 - <<'PY'
import os
import pathlib
import re
import subprocess
import sys

root = pathlib.Path(os.environ['FRAMEWORKS_DIR'])
available = {path.name for path in root.glob('*.framework') if path.is_dir()}
missing = set()
for framework in sorted(root.glob('*.framework')):
    for binary in framework.rglob('*'):
        if not binary.is_file() or not os.access(binary, os.X_OK):
            continue
        try:
            output = subprocess.run(['otool', '-L', str(binary)], check=True, capture_output=True, text=True).stdout
        except (subprocess.CalledProcessError, FileNotFoundError) as error:
            print(f'FRAMEWORK_INSPECTION_FAILED: {binary}: {error}', file=sys.stderr)
            sys.exit(1)
        for line in output.splitlines()[1:]:
            match = re.match(r'\s+@rpath/([^/]+\.framework)/', line)
            if match and match.group(1) not in available:
                missing.add((framework.name, match.group(1)))
if missing:
    for owner, dependency in sorted(missing):
        print(f'MISSING_FRAMEWORK_DEPENDENCY: {owner} requires {dependency}', file=sys.stderr)
    sys.exit(1)
if pathlib.Path(os.environ['FRAMEWORKS_DIR'], 'ExpoModulesWorklets.framework').is_dir():
    print('FRAMEWORK_MODE=precompiled-expo-modules')
else:
    print('FRAMEWORK_MODE=source-expo-modules')
print('FRAMEWORK_CLOSURE_OK')
PY
echo "RELEASE_APP=$APP"
