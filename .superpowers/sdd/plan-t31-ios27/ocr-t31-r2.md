Review complete: 10 finding(s) across 14 selected item(s).

─── apps/mobile/src/app/_layout.tsx:8-8 ───
[maintainability · low] 静态内联样式 `style={{ flex: 1 }}` 每次渲染都会新建样式对象，且属于非动态样式。按强制规则（避免静态 inline
style）及本项目既有约定（如 docs/plans/issue30-sweep/ocr/ocr-increment-b3.md 中对静态 inline style 移入 StyleSheet
的评审意见），建议提取为模块级 StyleSheet。注意：若采纳，需同步在 app-smoke.test.tsx 的 `react-native` stub 中补充 `StyleSheet: {
create: (styles) => styles }`，否则 `_layout.tsx` 在冒烟测试中会因 stub 缺少该导出而失败。

-       <SafeAreaView edges={['top', 'bottom']} style={{ flex: 1 }}>
+ import { StyleSheet } from 'react-native';
+ 
+ const styles = StyleSheet.create({ fill: { flex: 1 } });
+ 
+ /** The router hosts one Runtime-selected mobile surface at a time. */
+ export default function RootLayout() {
+   return (
+     <SafeAreaProvider>
+       <SafeAreaView edges={['top', 'bottom']} style={styles.fill}>


─── apps/mobile/src/app-smoke.test.tsx:94-95 ───
[test · low] 测试对 provider/safeView 都有带说明信息的 `assert.ok` 存在性断言，但对 `stack` 没有。若 `Stack`
元素缺失（被误删或改名），后续断言会以晦涩的 `deepEqual(undefined, { headerShown: false })` 失败，第 94 行的类型比较也会产生误导性
diff，无法直接定位根因。建议补一条显式断言，保持三者风格一致。

+   assert.ok(stack, 'Stack renders the routed screens');
    assert.equal((safeView.props.children as { type?: unknown } | undefined)?.type, stack?.type);
    assert.deepEqual(stack?.props.screenOptions, { headerShown: false });


─── apps/mobile/src/app-smoke.test.tsx:91-92 ───
[test · medium] 断言消息写的是 'SafeAreaProvider wraps routed content'，但实际只校验了 provider 在树中存在（descendants
是扁平遍历，不体现层级）。若未来布局退化为 provider 与 SafeAreaView 平级或包裹空内容（如
`<><SafeAreaView><Stack/></SafeAreaView><SafeAreaProvider/></>`），本测试仍全绿；而生产环境社区版 SafeAreaView 依赖上层
SafeAreaProvider（缺失时 useSafeAreaInsets 直接抛错），正是该冒烟测试应拦截的运行时崩溃。第 94 行已钉死 safeView→Stack
的直接嵌套，provider→safeView 这一层缺一条对称校验。

    assert.ok(provider, 'SafeAreaProvider wraps routed content');
+   assert.equal(
+     (provider.props.children as { type?: unknown } | undefined)?.type,
+     safeView?.type,
+     'SafeAreaProvider must be the outermost wrapper above SafeAreaView',
+   );
    assert.ok(safeView, 'SafeAreaView contains routed content');


─── apps/mobile/app.json:48-49 ───
[maintainability · medium] 移除 "./plugins/ios-xcode27" 注册后，apps/mobile/plugins/ios-xcode27.js（约 400
行）成为零引用文件：构建脚本已改为 clean prebuild + verify-ios-scene-project.ts，ios-acceptance-scripts.test.ts
也断言脚本不再引用它。按 t31 修复计划（docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md）该插件仅当"无活跃
config/script 声称其为生命周期事实源"时才可保留，但根 .gitignore:97 的注释仍写着"（ios-xcode27
插件负责生成内容）"，与本次变更后的事实不符。建议随本次迁移删除该插件文件（或至少同步修正 .gitignore 注释），避免后续维护者按过时说明理解 CNG 生成流程。



─── apps/mobile/package.json:11-11 ───
[maintainability · medium] 新增依赖 @expo/dom-webview 与 expo-constants
在全仓库（src、scripts、app.json、docs）除声明处无任何引用：@expo/dom-webview 是实验性 DOM 组件支持包，实际启用还需要
react-native-webview 宿主依赖（本包未声明，即使将来引用也无法工作）；expo-constants 仅作为 expo-router
等的传递依赖存在。建议移除这两个未使用依赖，待真正启用 DOM 组件时再随 react-native-webview 一并引入，避免扩大安装面并误导后续贡献者。



─── apps/mobile/scripts/verify-ios-framework-closure.py:60-61 ───
[bug · low] main() 中 `properties.get(...)` 位于 try/except 守护之外：当 Podfile.properties.json 的 JSON
根节点是数组/字符串等非对象（plistlib 分支根为数组同理）时，`json.load` 正常返回，但随后的 `properties.get` 会抛出未捕获的 AttributeError，输出裸
traceback 而非既定的 `FRAMEWORK_MODE_PROPERTIES_INVALID:` 错误口径。退出码仍为非零（fail-closed
语义保留），但构建日志排查时错误消息不统一。建议在 try 块后补一次 isinstance(properties, dict) 校验并复用同一错误码；顺带将 `__import__("json")`
改为顶部 `import json`，避免动态导入降低可读性。

+ import json  # 顶部
+ ...
+     if not isinstance(properties, dict):
+         fail(f"FRAMEWORK_MODE_PROPERTIES_INVALID: {properties_path}: root must be an object")
      source_value = properties.get("ios.buildReactNativeFromSource")
      expo_value = properties.get("EXPO_USE_PRECOMPILED_MODULES")


─── apps/mobile/app.json:48-48 ───
[bug] expo.plugins 数组的每个条目都必须解析为合法的 config plugin（包内需存在 app.plugin.js），否则 `npx expo prebuild` 会以
"Package \"expo-asset\" does not contain a valid config plugin" 硬失败。expo-asset 历来只是资源运行时库——仓库内 SDK
55 证据日志显示它一直作为 expo-audio/expo-router 的传递依赖参与 autolinking，从未出现在 plugins 中。而 ios-release-build.sh 第 1
步（`expo prebuild -p ios --clean --no-install`）是唯一受支持的 iOS 构建入口，若 SDK 57 的 expo-asset 仍不提供
app.plugin.js，T31 的 Release 流水线会在第一步中断。另外该条目没有任何测试覆盖（native-project-config.test.ts 只断言
expo-build-properties 配置）。若声明 expo-asset 是为了满足 pnpm 下 expo-router 的 peer 依赖，只需保留 package.json
中的依赖即可，不应写入 plugins；如确需保留请先实测一次 SDK 57 prebuild 通过并提供依据。

-       "expo-asset"
+       ]
+     ]


─── apps/mobile/src/plugins/ios-xcode27.test.ts:6-6 ───
[maintainability] 本文件已完全重写为针对 scripts/verify-ios-scene-project.ts 的生成工程契约测试，与 plugins/ios-xcode27.js
不再有任何关系，但仍位于 src/plugins/ 目录并沿用 ios-xcode27 文件名——读者（以及后续维护者）会误以为它覆盖
plugins/ios-xcode27.js（该插件文件现已零引用，属于前述已确认的死代码发现），同时针对 verify 脚本的测试却不在 src/scripts/ 惯例位置（对照
src/scripts/ios-acceptance-scripts.test.ts）。建议随本批清理一并将文件迁移为
src/scripts/verify-ios-scene-project.test.ts，使命名/位置与被测对象对齐，避免与死插件清理（confirmed finding 1）脱节后留下误导性命名。

+ // 将本文件重命名为 apps/mobile/src/scripts/verify-ios-scene-project.test.ts
  import { verifyIosSceneProject } from '../../scripts/verify-ios-scene-project.js';


─── apps/mobile/scripts/verify-ios-scene-project.ts:158-158 ───
[style · low] 这里出现了嵌套三元表达式（`current === "'" ? "'" : multiline ? '"""' :
'"'`），违反本仓库"禁止嵌套三元"的强制规约（verify 脚本属于工具代码，可读性建议同等适用）。建议改成 if/else 或提取小函数，语义不变：

-         delimiter = (current === "'" ? "'" : multiline ? '"""' : '"') + '#'.repeat(rawHashes);
+         let quote: string;
+         if (current === "'") quote = "'";
+         else if (multiline) quote = '"""';
+         else quote = '"';
+         delimiter = quote + '#'.repeat(rawHashes);


─── apps/mobile/src/ios-framework-closure.test.ts:133-134 ───
[test · low] 测试覆盖缺口：verify-ios-framework-closure.py 的 main 装载循环中，MALFORMED / UNSUPPORTED / MISSING /
OUTSIDE_BUNDLE / INSPECTION_FAILED 各 fail 分支都有对应用例，唯独 `FRAMEWORK_LOAD_OUTSIDE_APP`（load command 借
`..` 解析出 .app 之外的逃逸守卫）没有用例。这是安全相关的 fail-closed 守卫，建议补一组用例锁住该行为：

- test('production checker fails closed on unsupported non-system framework path tokens', () => {
-   for (const load of ['@unknown_path/Foo.framework/Foo', '/private/other/Foo.framework/Foo']) {
+ test('production checker rejects load commands escaping the app bundle', () => {
+   for (const load of [
+     '@executable_path/../Missing.framework/Missing',
+     '@rpath/../../Missing.framework/Missing',
+   ]) {
+     const f = fixture({ app: load });
+     try {
+       const result = run(f);
+       assert.notEqual(result.status, 0, `${load} unexpectedly passed: ${result.output}`);
+       assert.match(result.output, /FRAMEWORK_LOAD_OUTSIDE_APP: WeKnora\.app/);
+     } finally { rmSync(f.root, { recursive: true, force: true }); }
+   }
+ });


LLM retry report summary: 1 of 55 requests affected -- 1 request cancelled

Context compaction (1 request):
- apps/mobile/src/app-smoke.test.tsx,apps/mobile/src/app/_layout.tsx: cancelled

Per-attempt detail: --format json (retry_report).
