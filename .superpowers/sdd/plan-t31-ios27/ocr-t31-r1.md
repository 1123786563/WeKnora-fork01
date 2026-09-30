Review complete: 6 finding(s) across 7 selected item(s).

─── apps/mobile/src/app/_layout.tsx:8-8 ───
[style · low] 静态内联样式 `style={{ flex: 1 }}` 不符合“避免内联 style，仅动态样式可用内联”的规范。flex: 1
是固定值，每次渲染都会创建新的样式对象，建议提取为模块级 StyleSheet.create 静态样式，既符合规范也便于复用。

-       <SafeAreaView edges={['top', 'bottom']} style={{ flex: 1 }}>
+ import { StyleSheet } from 'react-native';
+ 
+ const styles = StyleSheet.create({
+   container: { flex: 1 },
+ });
+ 
+ // ...
+ <SafeAreaView edges={['top', 'bottom']} style={styles.container}>


─── apps/mobile/app.json:16-16 ───
[bug · medium] `expo.ios.deploymentTarget` 不是 app.json `expo.ios` 节点的有效键:已知 Expo 版本中,iOS 部署目标只能通过
`expo-build-properties` 插件的 `ios.deploymentTarget` 写入 prebuild 产物(它会同时写入 Podfile.properties.json 的
`ios.deploymentTarget` 与 pbxproj 的 `IPHONEOS_DEPLOYMENT_TARGET`)。原先负责写入该值的本地插件
`./plugins/ios-xcode27`(withPodfileProperties)已从 plugins 移除,而新的 expo-build-properties 配置块中没有
`deploymentTarget`,若该顶层键被 prebuild 静默忽略,scripts/verify-ios-scene-project.ts 中『Podfile properties
deployment target must be 16.4』与『All app target deployment settings must be 16.4』的守卫将失败。建议将
`deploymentTarget` 并入 expo-build-properties 的 ios 块以确保两个校验点都被覆盖(同时需同步更新
src/native-project-config.test.ts 中的断言)。

+ [
+   "expo-build-properties",
+   {
+     "ios": {
+       "enableSceneSupport": true,
+       "buildReactNativeFromSource": true,
+       "usePrecompiledModules": false,
        "deploymentTarget": "16.4"
+     }
+   }
+ ]


─── apps/mobile/scripts/verify-ios-scene-project.ts:43-43 ───
[bug · critical] configIds 正则与真实 pbxproj 格式不匹配,校验在真实产物上必然失败。真实 Xcode/Expo prebuild 生成的
project.pbxproj 中,XCConfigurationList 的各构建配置成员是 `ID /* name */ = { ... };` 分号结尾的赋值(OpenStep plist
格式中逗号只用于 `(...)` 数组,而 XCConfigurationList 没有数组字段),不存在夹具中的 `buildConfigurations = ( C1 /* Debug */,
... )` 逗号数组写法。当前前瞻 `(?=\s*(?:\/\*[^*]*\*\/\s*)?[,])` 要求 ID 后跟逗号,在真实产物上只能匹配到 buildSettings
数组字面量里的杂散十六进制片段或直接为空,导致 `configIds.length === 0` 恒真、恒报 'All app target deployment settings must be
16.4',ios-release-build.sh 第 1.5 步在 set -euo pipefail 下无条件 exit 1,iOS Release 构建流水线被新门禁整体阻断(与 Issue
#31 的 Release 打包验收目标冲突)。配套测试 src/plugins/ios-xcode27.test.ts 的夹具按本正则反推构造了不存在的 pbxproj
格式,使缺陷无法被现有测试捕获。建议改为按「ID + 可选注释 + = + {」提取成员 ID,并把夹具改成真实分号赋值格式(更稳妥的做法是用一次真实 prebuild 产出的 pbxproj
快照作夹具)。

-   const configIds = [...(configListBlock ?? '').matchAll(/([A-F0-9]+)(?=\s*(?:\/\*[^*]*\*\/\s*)?[,])/g)].map((match) => match[1]);
+   // 真实 pbxproj 的 XCConfigurationList 中,各构建配置是 `ID /* name */ = { ... };` 分号赋值成员(无逗号数组),
+   // 按「ID + 可选注释 + '=' + '{'」提取;限定 24 位十六进制避免误匹配 buildSettings 数组字面量中的杂散片段。
+   const configIds = [...(configListBlock ?? '').matchAll(/([A-F0-9]{24})(?=\s*(?:\/\*[^*]*\*\/\s*)?=\s*\{)/g)].map((match) => match[1]);


─── apps/mobile/package.json:11-11 ───
[maintainability · low] @expo/dom-webview 在本仓库无任何使用点(apps/mobile 源码、app.json plugins、packages/
下均无引用),且其配套的 react-native-webview 也未安装,属于未使用依赖。本次升级已把整组 Expo 依赖迁到 SDK
57,多一个孤立依赖会扩大后续同步升级与安全维护面。建议随实际引入 WebView 的改动一并添加;若是为后续需求预留,请加注释说明用途。

-     "@expo/dom-webview": "~57.0.1",
+ // 若无近期使用计划,移除该依赖:
+ // (其余依赖保持不变)


─── apps/mobile/scripts/verify-ios-scene-project.ts:5-5 ───
[test · medium] 新守卫的全部逻辑(含 argv 驱动的 readFileSync 解析生成的 plist/pbxproj/AppDelegate)都放在
scripts/,违背了仓库自己的布局裁决:scripts/verify-ios-native-deps.ts 头注释明确记录「Mimosa path-traversal 规则禁止
argv-readFileSync 留在 scripts/,逻辑须放 src/ 以便单测」,ios-release-build.sh 第 52 行也以此为卖点(「逻辑与单测在
src/ios-native-deps.ts,7 用例,布局同 mimosa 裁决 3」)。实际后果:(1) tsconfig 的 include 只有 src/** 与
.expo/types,本文件不会被 `pnpm typecheck` 检查;(2) 测试脚本 glob 为 `tsx --test
'src/**/*.test.ts*'`,本文件零单测——而这恰是全仓库正则最密集、最易碎的漂移守卫(configIds 正则问题即为例证)。建议照 verify-ios-native-deps
的模式:逻辑迁到 src/ios-scene-project.ts 并补单测,scripts/ 只留薄 CLI 壳;或将 "scripts/**/*.ts" 加入 tsconfig include。

- export function verifyIosSceneProject(iosDirectory: string): string[] {
+ // scripts/verify-ios-scene-project.ts —— 薄 CLI 壳,逻辑与单测放 src/(对齐 mimosa 裁决与 verify-ios-native-deps 布局)
+ import { main } from '../src/ios-scene-project-cli.ts';
+ 
+ main();
+ // 逻辑与 7+ 个夹具单测迁至 src/ios-scene-project.ts + src/ios-scene-project.test.ts


─── apps/mobile/scripts/verify-ios-scene-project.ts:158-158 ───
[style · low] 嵌套三元表达式(外层三元的假值分支又是一个三元),违反项目「禁止嵌套三元」规则,且这段字符串定界符推导本身是 lexer 中最易读错的地方。建议拆成独立小函数或
if/else 链。

-         delimiter = (current === "'" ? "'" : multiline ? '"""' : '"') + '#'.repeat(rawHashes);
+         const quoteDelimiter = current === "'" ? "'" : multiline ? '"""' : '"';
+         delimiter = quoteDelimiter + '#'.repeat(rawHashes);
+         // 或改为: delimiter = stringDelimiterFor(current, multiline) + '#'.repeat(rawHashes);

