#!/usr/bin/env bash
# T39（Issue #69）：可复现的 iOS 模拟器 Release 构建管线（幂等，可反复重跑）。
# 前置：macOS + Xcode 27、根目录已 pnpm install、CocoaPods 可用。
# 用法：bash apps/mobile/scripts/ios-release-build.sh [仓库根，默认当前目录]
# 产物：apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app
#       docs/plans/issue30-sweep/ios-evidence/t39/xcodebuild-release.log
set -euo pipefail

ROOT="${1:-$(pwd)}"
MOBILE="$ROOT/apps/mobile"
IOS="$MOBILE/ios"
EVIDENCE_DIR="$ROOT/docs/plans/issue30-sweep/ios-evidence/t39"
mkdir -p "$EVIDENCE_DIR"
cd "$MOBILE"

# 1) prebuild 重新生成 ios/（.gitignore 不入库）并重放入库 plugin
#    apps/mobile/plugins/ios-xcode27.js（scene 生命周期、splash wordmark、
#    Podfile 部署目标钳制与告警抑制）——漂移防护的唯一事实源。
npx expo prebuild -p ios --no-install

# 2) pnpm 工作区下 babel-preset-expo 的符号链接在干净安装后缺失
#    （B3 实测：Metro「Cannot find module 'babel-preset-expo'」失败于 RN bundle 阶段）——缺则重建。
if [ ! -e "$MOBILE/node_modules/babel-preset-expo" ]; then
  preset="$(ls -d "$MOBILE/node_modules/.pnpm"/babel-preset-expo@*/node_modules/babel-preset-expo 2>/dev/null | head -1)"
  if [ -z "$preset" ]; then echo "babel-preset-expo not found in the pnpm store; run pnpm install first" >&2; exit 1; fi
  ln -s "$preset" "$MOBILE/node_modules/babel-preset-expo"
fi

# 3) Pods（含 plugin 写入的部署目标钳制与 inhibit_all_warnings!）
cd "$IOS"
pod install

# 4) 模拟器 Release 全量构建。注意：build/ 已存在时无需删除；确需删除则必须回到
#    第 3 步重建（codegen 生成源码在 build/generated/ios/ReactCodegen 下，B4 实测）。
xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -sdk iphonesimulator \
  -configuration Release -derivedDataPath build \
  -destination 'generic/platform=iOS Simulator' build 2>&1 | tee "$EVIDENCE_DIR/xcodebuild-release.log"

APP="$IOS/build/Build/Products/Release-iphonesimulator/WeKnora.app"
test -d "$APP" || { echo "expected Release app missing: $APP" >&2; exit 1; }
echo "RELEASE_APP=$APP"
