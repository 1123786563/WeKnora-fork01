/** iOS 原生依赖漂移防护 CLI（src 实现，薄壳在 apps/mobile/scripts/verify-ios-native-deps.ts）。
 *  用法（ios-release-build.sh 在 pod install 后自动调用）：
 *    cd apps/mobile && pnpm exec tsx scripts/verify-ios-native-deps.ts
 *  流程：expo-modules-autolinking resolve（node_modules 现状）→ 读 ios/Podfile.lock →
 *  nativePodGaps 判定；有缺口打印清单并 exit 1（阻塞 xcodebuild，省下一次 20 分钟的
 *  静默缺模块构建），全绿打印 IOS_NATIVE_DEPS_OK。
 *  无 argv 路径输入：全部路径相对本文件推导，Mimosa 路径穿越三件套不适用（同
 *  ios-release-evidence-cli.ts 的裁决 3 布局）。 */
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { nativePodGaps, parseAutolinkingResolution } from './ios-native-deps.ts';

const MOBILE_DIR = dirname(dirname(fileURLToPath(import.meta.url)));

export function verifyFromInputs(resolutionJson: string, podfileLock: string): { gaps: ReturnType<typeof nativePodGaps>; resolutionCount: number } {
  const resolution = parseAutolinkingResolution(resolutionJson);
  return { gaps: nativePodGaps(resolution, podfileLock), resolutionCount: resolution.length };
}

export function main(): void {
  // autolinking 以 node_modules 为事实源；cwd 必须是 apps/mobile（脚本约定）。
  const resolutionJson = execFileSync(
    'npx',
    ['expo-modules-autolinking', 'resolve', '--platform', 'ios', '--json'],
    { cwd: MOBILE_DIR, encoding: 'utf8', stdio: ['ignore', 'pipe', 'inherit'] },
  );
  const lockPath = join(MOBILE_DIR, 'ios', 'Podfile.lock');
  let podfileLock: string;
  try {
    podfileLock = readFileSync(lockPath, 'utf8');
  } catch {
    console.error(`ios/Podfile.lock not found at ${lockPath} — run ios-release-build.sh (prebuild + pod install) before building`);
    process.exit(1);
  }
  const { gaps, resolutionCount } = verifyFromInputs(resolutionJson, podfileLock);
  if (gaps.length > 0) {
    console.error(`stale Podfile.lock: ${gaps.length} native pod(s) resolved by expo-modules-autolinking but missing from Podfile.lock PODS section:`);
    for (const gap of gaps) console.error(`  - ${gap.packageName} -> ${gap.pod}`);
    console.error('Rebuild via apps/mobile/scripts/ios-release-build.sh (prebuild + pod install). A bare xcodebuild on a leftover ios/ tree would silently produce an app without these native modules (B5 recheck finding 1).');
    process.exit(1);
  }
  const podCount = [...new Set(parseAutolinkingResolution(resolutionJson).flatMap((m) => m.pods))].length;
  console.log(`IOS_NATIVE_DEPS_OK modules=${resolutionCount} pods=${podCount}`);
}
