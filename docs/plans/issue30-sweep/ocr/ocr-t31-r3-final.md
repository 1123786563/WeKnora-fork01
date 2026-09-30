Review partially complete: 3 finding(s); 1 of 8 selected item(s) failed.

─── apps/mobile/scripts/verify-ios-framework-closure.py:88-94 ───
[bug · medium] 闭包校验对非 framework 形态的加载命令存在静默放行缺口：当加载项匹配 @token 但不含 `.framework` 组件（如
`@rpath/libFoo.dylib`、`@loader_path/...dylib`），或为非 `/System/Library/Frameworks/` 开头的绝对路径 dylib（如
`/usr/lib/swift/...dylib` 之外的任意绝对路径）时，`not token_match or not match` 分支内两个 fail 都被 `if component`
短路，最终 `continue` 直接跳过。这违背了模块 docstring 宣称的 fail-closed 契约——一个缺失的 `.dylib` 动态依赖（dyld 启动即崩，与该守卫要拦截的 R2
故障同类）不会被检出，守卫通过。建议：对未被识别的非系统加载项同样 fail closed（系统根路径如 /System/Library/、/usr/lib/ 白名单放行），或将 `.dylib`
形态按与 framework 相同方式对 bundle 内容做闭包解析。

              if not token_match or not match:
                  framework_name = component.group("framework") if component else load
                  if component and (load.endswith(".framework") or load.endswith(".framework/")):
                      fail(f"MALFORMED_FRAMEWORK_LOAD_PATH: {owner} requires {framework_name}: {load}")
                  if component:
                      fail(f"UNSUPPORTED_FRAMEWORK_LOAD_PATH: {owner}: {load}")
+                 if not load.startswith(("/System/Library/", "/usr/lib/")):
+                     fail(f"UNSUPPORTED_DYNAMIC_LIBRARY_LOAD: {owner}: {load}")
                  continue


─── apps/mobile/scripts/ios-release-build.sh:30-31 ───
[maintainability · low] 新增的 `--clean` 使第 1 步每次运行都整目录删除 ios/（含 ios/build 及其 codegen 产物
build/generated/ios/ReactCodegen），而第 4 步注释仍保留旧流程的指引「build/ 已存在时无需删除；确需删除则必须回到第 3 步重建」。在本脚本当前流程下
build/ 永远不可能预先存在，该操作性提示已失效且与新行为矛盾，会误导后续维护者以为存在增量构建路径。建议同步更新第 4 步注释，说明每次均为 --clean 后的全量重建。

  # 1) clean prebuild 丢弃任何忽略的旧 SDK 输出并重新生成 ios/（.gitignore 不入库）。
+ #    注意 --clean 会整目录删除 ios/，包括 ios/build 与其中的 codegen 产物，故第 4 步每次都是全量重建。
  npx expo prebuild -p ios --clean --no-install


─── apps/mobile/scripts/verify-ios-scene-project.ts:26-29 ───
[bug · low] ExpoReactNativeFactoryProvider 契约检查直接作用于未屏蔽注释/字符串的原始 appDelegate 文本，而本文件对
RCTLinkingManager 两项检查特意先经 maskSwiftNonCode 屏蔽注释后再匹配（正是为了防注释伪装满足契约）。若 SDK 模板漂移导致真实 conformance
消失、但残留注释仍含该 token（如 `// TODO: ExpoReactNativeFactoryProvider`），此项检查会静默通过，形成 fail-closed
门禁中的假阴性。建议改为对屏蔽后的 swiftCode 检查（与文件自身做法保持一致）：

-   if (!appDelegate.includes('ExpoReactNativeFactoryProvider')) {
+   const swiftCode = maskSwiftNonCode(appDelegate);
+   if (!swiftCode?.includes('ExpoReactNativeFactoryProvider')) {
      issues.push('AppDelegate must conform to ExpoReactNativeFactoryProvider');
    }
-   const swiftCode = maskSwiftNonCode(appDelegate);


LLM retry report summary: 1 of 43 requests affected -- 1 request failed

Core review (1 request):
- apps/mobile/src/app/_layout.tsx: timed out -> failed

Per-attempt detail: --format json (retry_report).
