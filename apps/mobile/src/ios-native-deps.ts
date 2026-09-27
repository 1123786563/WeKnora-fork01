/** iOS 原生依赖漂移防护（B5 复验 important 发现的固化，2026-09-28）。
 *
 * 陷阱：apps/mobile/ios/ 不入库（根 .gitignore 忽略，CNG/prebuild 流程），本地遗留的旧
 * Podfile.lock 不会因 package.json 新增 expo-* 依赖而失效——直接对遗留工程跑 xcodebuild
 * 增量构建照样 BUILD SUCCEEDED，但产物缺 ExpoAudio/ExpoNetwork 等原生模块，只在运行时
 * require('expo-audio') 处崩溃（B5 复验实证：Podfile.lock 缺 ExpoAudio/ExpoNetwork、
 * 二进制 0 符号；重放 prebuild+pod install 后 1414/83 符号）。
 *
 * 本模块把「Podfile.lock 覆盖 autolinking 解析出的全部原生 pod」变成机器可检查的不变量：
 * 事实源取 expo-modules-autolinking 的解析结果（node_modules 现状），逐一对照
 * Podfile.lock 的 PODS 节（该节是已安装 pod 的扁平全清单，出现即已安装）。
 * 纯函数、无 IO，CLI 壳在 scripts/verify-ios-native-deps.ts（布局沿用
 * mimosa-adjudications.md 裁决 3：读取/子进程在 src/，scripts/ 仅薄壳）。 */

export interface ResolvedNativeModule {
  packageName: string;
  pods: readonly string[];
}

/** 解析 `expo-modules-autolinking resolve --platform ios --json` 输出。
 *  形如 {"modules":[{"packageName":"expo-audio","pods":[{"podName":"ExpoAudio", …}], …}]}
 *  （一个包可对应多个 pod，如 expo-modules-core → ExpoModulesCore + ExpoModulesJSI；
 *  pods 为空的包不产出条目。） */
export function parseAutolinkingResolution(json: string): readonly ResolvedNativeModule[] {
  const parsed = JSON.parse(json) as {
    modules?: ReadonlyArray<{ packageName?: unknown; pods?: ReadonlyArray<{ podName?: unknown }> }>;
  };
  const out: ResolvedNativeModule[] = [];
  for (const module of parsed.modules ?? []) {
    if (typeof module.packageName !== 'string' || module.packageName === '') continue;
    const pods = (module.pods ?? [])
      .map((pod) => (typeof pod.podName === 'string' ? pod.podName : ''))
      .filter((podName) => podName !== '');
    if (pods.length === 0) continue;
    out.push({ packageName: module.packageName, pods });
  }
  return out;
}

/** 取 Podfile.lock 的 PODS: 节（从文件头到首个顶层小节 DEPENDENCIES: 之间）。
 *  PODS 节是 CocoaPods 写出的已安装 pod 扁平清单；DEPENDENCIES 及之后的
 *  SPEC CHECKSUMS / EXTERNAL SOURCES 节不参与判定，避免把「声明过」误当「装上了」。 */
export function podsSectionOf(podfileLock: string): string {
  const end = podfileLock.indexOf('\nDEPENDENCIES:');
  return end < 0 ? podfileLock : podfileLock.slice(0, end);
}

/** 从 PODS 节提取已安装 pod 名。只认两空格缩进的条目行 `  - ExpoAudio (55.0.18):`，
 *  深缩进（四空格）的传递依赖行与 subspec 行不计——顶层条目在即已安装，足够判定。 */
export function installedPodNames(podsSection: string): ReadonlySet<string> {
  const names = new Set<string>();
  for (const match of podsSection.matchAll(/^ {2}- "?([^\s"(]+) \(/gm)) names.add(match[1]);
  return names;
}

export interface NativePodGap {
  packageName: string;
  pod: string;
}

/** 不变量检查：autolinking 解析出的每个原生 pod 必须出现在 Podfile.lock 的 PODS 节。
 *  返回缺失清单，空数组 = 通过。这正是 B5 陷阱的机器化：旧 lock 缺新模块时在此点名，
 *  而不是让 xcodebuild 静默成功、运行时才崩。 */
export function nativePodGaps(
  resolution: readonly ResolvedNativeModule[],
  podfileLock: string,
): readonly NativePodGap[] {
  const installed = installedPodNames(podsSectionOf(podfileLock));
  const gaps: NativePodGap[] = [];
  for (const module of resolution) {
    for (const pod of module.pods) {
      if (!installed.has(pod)) gaps.push({ packageName: module.packageName, pod });
    }
  }
  return gaps;
}
