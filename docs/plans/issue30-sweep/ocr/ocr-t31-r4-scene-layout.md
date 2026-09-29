Review complete: 6 finding(s) across 3 selected item(s).

─── apps/mobile/scripts/verify-ios-scene-project.ts:116-120 ───
[bug · low] parsePlist 的 string/integer 分支假定"恰好一个文本 token + 闭合标签"：遇到空值 <string></string> 时会把闭合标签
</string> 当作 value 消费，再用 index++ 吞掉下一个 token，导致后续 token 整体错位；自闭合 <string/>（tokenizer 会产出一个 token）以及
<real>/<data>/<date> 值会直接落入 throw，使 parsePlist 返回 undefined。这两种情况最终都会被误报为 'scene role must map to
EXExpoAppSceneDelegate'——虽是 fail-closed（不会误通过），但错误归因会误导排障。建议取值前先判断下一个 token 是否为闭合标签以支持空值：

      if (token === '<string>' || token === '<integer>') {
+       const closing = token === '<string>' ? '</string>' : '</integer>';
+       if (tokens[index]?.trim() === closing) { index++; return ''; }
        const value = tokens[index++]?.trim() ?? '';
        index++;
        return value;
      }


─── apps/mobile/scripts/verify-ios-scene-project.ts:50-51 ───
[maintainability · low] 结构解析失败与契约值漂移共用同一条报错：当 targetBlock/configListBlock 未命中（如 pbxproj 对象 ID
变为小写、段落结构变化、文件内容被替换为无 section 形式）时 configIds 为空，文案却断言"部署目标必须是
16.4"，把"无法解析工程结构"与"值不符合契约"混为一谈。同类问题也出现在关键文件缺失时：read 已记录 missing generated file，后续
provider/open-url/universal-link/Podfile JSON 校验仍会追加级联报错，产生噪音。建议 configIds 为空时单独报 "unable to locate
WeKnora build configuration list" 类错误，并在核心文件缺失时短路返回，提升 CI 排障效率：

-   if (configIds.length === 0 || deploymentValues.some((value) => value !== '16.4')) {
-     issues.push('All app target deployment settings must be 16.4');
+   if (configIds.length === 0) {
+     issues.push('Unable to locate WeKnora build configuration list in project.pbxproj');
+   } else if (deploymentValues.some((value) => value !== IOS_DEPLOYMENT_TARGET)) {
+     issues.push(`All app target deployment settings must be ${IOS_DEPLOYMENT_TARGET}`);
+   }


─── apps/mobile/scripts/verify-ios-scene-project.ts:76-86 ───
[maintainability · low] pbxBlock 与 pbxDictionary
各自维护一套几乎相同的"定位声明后按深度配对花括号"扫描逻辑（仅声明正则前缀不同），日后若需处理引号内的花括号等边界，两处容易不同步。建议提取公共的 balanced-brace
提取函数，两个函数保留各自的声明定位正则后复用：

- function pbxDictionary(source: string, key: string): string | undefined {
-   const declaration = new RegExp(`(?:^|[,{\\n])\\s*${key}\\s*=\\s*\\{`).exec(source);
-   if (!declaration) return undefined;
-   const open = source.indexOf('{', declaration.index + declaration[0].length - 1);
+ function balancedBraceBlock(source: string, searchFrom: number): string | undefined {
+   const open = source.indexOf('{', searchFrom);
+   if (open < 0) return undefined;
    let depth = 1;
    for (let index = open + 1; index < source.length; index++) {
      if (source[index] === '{') depth++;
      if (source[index] === '}' && --depth === 0) return source.slice(open + 1, index);
    }
    return undefined;
  }
+ // pbxBlock / pbxDictionary 保留各自的声明定位正则，命中后统一调用 balancedBraceBlock 提取区块内容。


─── apps/mobile/scripts/verify-ios-scene-project.ts:55-55 ───
[maintainability · low] 契约字面量散落重复：'16.4' 在部署目标比较、Podfile 比较及两条报错文案中共出现 4
次，'EXExpoAppSceneDelegate'、'RCTLinkingManager'、'WeKnora' 等契约字符串也内联多处。一旦部署目标升级（如
17.0）或应用改名，需在同一文件内多处同步（还需与 app.json 的 expo.ios.deploymentTarget 联动）。建议在文件顶部集中声明常量并在比较与文案中复用：

-     if (properties['ios.deploymentTarget'] !== '16.4') issues.push('Podfile properties deployment target must be 16.4');
+ const IOS_DEPLOYMENT_TARGET = '16.4';
+ // ...
+ if (properties['ios.deploymentTarget'] !== IOS_DEPLOYMENT_TARGET) {
+   issues.push(`Podfile properties deployment target must be ${IOS_DEPLOYMENT_TARGET}`);
+ }


─── apps/mobile/scripts/verify-ios-scene-project.ts:30-37 ───
[maintainability · low] open-URL 与 universal-link 两段校验是完全相同的模式（swiftMethodBody 提取 +
RCTLinkingManager 转发正则 + 报错文案），仅签名正则和 label 不同。与 pbxBlock/pbxDictionary 的重复属同类问题但位置不同：若后续新增回调（如
shortcut）或调整转发判定（例如改为同时接受 super 转发），需要在两处同步修改，容易漏改造成两个回调校验行为漂移。建议提取一个局部 helper 统一"提取方法体并要求转发到
RCTLinkingManager"的判定（保留现有 'open-URL callback' / 'universal-link callback' 文案子串以兼容现有测试断言）：

-   const openUrlBody = swiftCode && swiftMethodBody(swiftCode, /\boverride\s+func\s+application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*open\s+\w+\s*:\s*URL\b/);
-   if (!openUrlBody || !/\bRCTLinkingManager\s*\.\s*application\s*\(/.test(openUrlBody)) {
-     issues.push('AppDelegate open-URL callback must forward to RCTLinkingManager.application');
-   }
-   const universalLinkBody = swiftCode && swiftMethodBody(swiftCode, /\boverride\s+func\s+application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*continue\s+\w+\s*:\s*NSUserActivity\b/);
-   if (!universalLinkBody || !/\bRCTLinkingManager\s*\.\s*application\s*\(/.test(universalLinkBody)) {
-     issues.push('AppDelegate universal-link callback must forward to RCTLinkingManager.application');
+   const requireLinkingForward = (label: string, signature: RegExp) => {
+     const body = swiftCode ? swiftMethodBody(swiftCode, signature) : undefined;
+     if (!body || !/\bRCTLinkingManager\s*\.\s*application\s*\(/.test(body)) {
+       issues.push(`AppDelegate ${label} callback must forward to RCTLinkingManager.application`);
-   }
+     }
+   };
+   requireLinkingForward('open-URL', /\boverride\s+func\s+application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*open\s+\w+\s*:\s*URL\b/);
+   requireLinkingForward('universal-link', /\boverride\s+func\s+application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*continue\s+\w+\s*:\s*NSUserActivity\b/);


─── apps/mobile/src/app/_layout.tsx:9-9 ───
[style · low] `style={{ flex: 1 }}` 和 `edges={['top', 'bottom']}`
都是静态配置,建议提升为模块级常量,避免每次渲染重建对象,同时符合“避免内联静态样式”的规范(RootLayout 重渲染频率低,性能影响有限,主要为一致性)。注意:提升为常量后现有 smoke
test 中 `deepEqual(safeView.props.edges, ['top','bottom'])` 断言仍可通过。

-       <SafeAreaView edges={['top', 'bottom']} style={{ flex: 1 }}>
+ const ROOT_EDGES: Edge[] = ['top', 'bottom'];
+ const ROOT_STYLE = { flex: 1 } as const;
+ 
+ // ...
+ <SafeAreaView edges={ROOT_EDGES} style={ROOT_STYLE}>

