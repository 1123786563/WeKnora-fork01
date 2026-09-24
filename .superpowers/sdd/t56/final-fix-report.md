# T56 最终审查修复报告（issue30-sweep / #56 听写）

日期：2026-09-25　worktree：`.worktrees/issue30-sweep-t56`　基线：`31d9e5b12`

最终审查共 3 项 minor 发现，本次全部处理完毕。每项的处置、证据与回归如下。

---

## 发现 1（minor）：计划基线段「19 个测试函数」实为 18 —— 已修正（纯计划元数据）

**审查原文**：计划基线段声称 Go 语音测试命中「19 个测试函数」，实跑 `-v` 计数为 18 个顶层测试函数，全部 PASS、0 FAIL，纯计划元数据偏差。

**处置**：修正 `docs/plans/issue30-sweep/plans/plan-t56.md` 中全部 4 处「19 个」计数声明（Architecture 行、Tech Stack 基线行、AC3 blocked-env 行、差异记录第 3 条），并在差异记录第 3 条留下勘误注记（含本次实跑命令与基线 HEAD 事实）。

**本次实跑证据**（本 worktree，HEAD `31d9e5b12`）：

```
$ grep -cE '^func (TestVoice|TestTranscription|TestPriceVersion)' internal/handler/mobile_voice_test.go
18
$ grep -rnE '^func (TestVoice|TestTranscription|TestPriceVersion)' internal/handler/
# 18 条命中，全部位于 mobile_voice_test.go:313-957，其他文件 0 条
$ go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	1.815s
$ go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion' -v -count=1 | grep -cE '^=== RUN   Test'
18
$ go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion' -v -count=1 | grep -cE '^--- FAIL'
0
```

**补充事实**：计划基线 HEAD `fb5f6653a` 当时该计数同样是 18（`git show fb5f6653a:internal/handler/mobile_voice_test.go | grep -cE …` = 18）——「19」自计划撰写起即笔误，并非后续测试增删。18 个测试函数首列为 `TestVoiceSessionHappyPathTokenNeverLogged`（`internal/handler/mobile_voice_test.go:313`），与计划所引一致。

---

## 发现 2（minor）：`MobileVoiceTranscriptionResult.replay` 为死字段 —— 已移除（附回归测试）

**审查原文**：服务端成功信封只含 `{text,audio_seconds,settled}`（`internal/handler/mobile_voice.go:601` 实核），`replay` 永不为 true，系前向兼容占位，无害。

**处置**：删除而非保留。理由：信封映射分支 `...(data.replay === true ? { replay: true } : {})` 对真实服务端是不可达死代码；grep 证实无任何消费方（mobile-core `DictationTranscriptionResult` 仅 `{text, audioSeconds}`，apps/mobile 零引用）。移除后客户端结果面与服务端信封**逐字对齐**，比「注释提示勿依赖」更彻底地满足审查意图。

**改动**：

- `packages/api-client/src/mobile/voice.ts`：接口删 `replay?: boolean`（现 :28-36，附对齐说明注释）；信封 cast 删 `replay?: unknown`（现 :98）；结果映射删 `replay` 分支（现 :101-103）。
- `packages/api-client/src/mobile/voice.test.ts:34`：voice_session_id 用例的 mock 信封 `{text:'x', replay:true}` → `{text:'x', settled:true}`（对齐真实信封）。
- `packages/api-client/src/mobile/voice.test.ts:92-104`：**新增回归测试** `the result surface is exactly the server envelope fields (unknown fields never pass through)`——即使信封出现未知/遗留字段（mock 注入 `replay: true`），结果面也 `deepEqual({text})` 且 `'replay' in result === false`，钉死「未知信封字段不透传」。
- `docs/plans/issue30-sweep/plans/plan-t56.md`：Produces 行（原 :731）删 `replay?: boolean` 并附移除缘由；Task 2 内嵌测试/实现模板 4 处（原 :774/:874/:936/:942）同步删除，保持计划↔代码一致，防止后续读者从计划模板重新引入。

**回归输出**：

```
$ pnpm exec tsx --test packages/api-client/src/mobile/voice.test.ts
# tests 7  # pass 7  # fail 0        （6 既有 + 1 新增）
```

---

## 发现 3（minor）：原生捕获 Adapter 未显式删临时文件 —— 语义诚实化（注释修正），显式 unlink 不在本批实施

**审查原文**：stop() 成功路径 release() 录音机后，uri 临时文件的删除依赖 expo-audio release/系统回收；CONTEXT.md:339 的客户端侧由 dropIntent+release 承担，平台真实行为属 #69/#70 真机验收（计划已显式声明），本地无法验证真实删除。

**处置**：修正代码中的**过度声明注释**，不新增显式 unlink。原 `dictation-capture.ts` cancel() 注释「释放即弃临时文件」暗示 release 会删盘，与事实不符。现改为如实声明：

- `apps/mobile/src/adapters/dictation-capture.ts:52-54`（stop 路径）：注明 uri 指向的临时文件在 stop() 返回后**必须存活**——模块要用它发起 multipart 上载；上传完成后的删除依赖平台回收，真机实际删除验收属 #69/#70。
- `apps/mobile/src/adapters/dictation-capture.ts:70-74`（cancel 路径）：注明 CONTEXT.md:339 客户端侧由「模块 dropIntent 弃内存引用 + 此处 release 释放录音机对象」承担；**release 不保证即时删盘**；显式 unlink 需引入 `expo-file-system` 原生依赖（当前 `apps/mobile/package.json` 无此依赖，`expo-audio` 同样仅列于真机前置），超出 #56 计划声明范围。
- `docs/plans/issue30-sweep/plans/plan-t56.md` Global Constraints 对应行（原 :32）补注「release 释放录音机对象，不保证即时删盘——真机实际删除验收属 #69/#70」。

**为何不在本批加显式删除**（如实说明）：① `expo-file-system` 不在 apps/mobile 依赖中，引入即新增真机构建前置，超出审查 finding 与计划的声明范围；② 真机上「文件确被删除」本地不可验证（spec Testing Decisions：「real-device acceptance separately」，审查原文亦确认），Node 链只能断言 spy 被调用，属伪造平台结论；③ 本批行为零变更——客户端侧义务链（dropIntent + release + 平台回收）保持计划声明原样，#69/#70 真机验收时再决定是否加显式 unlink。

**回归输出**（注释级改动，行为零变更）：

```
$ pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts packages/api-client/src/mobile/voice.test.ts apps/mobile/src/dictation-view.test.ts apps/mobile/src/adapters/dictation-capture.test.ts
# tests 30  # pass 30  # fail 0
```

---

## 全量回归（本次实跑）

| 检查 | 命令 | 结果 |
|---|---|---|
| Go 语音基线（发现 1 证据） | `go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion' -count=1` | **ok**（1.815s） |
| Go -v 顶层计数 | 同上 `-v` + `grep -cE '^=== RUN   Test'` | **18 / 0 FAIL** |
| api-client voice 契约 | `pnpm exec tsx --test packages/api-client/src/mobile/voice.test.ts` | **7 pass / 0 fail** |
| 听写四文件 | `pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts packages/api-client/src/mobile/voice.test.ts apps/mobile/src/dictation-view.test.ts apps/mobile/src/adapters/dictation-capture.test.ts` | **30 pass / 0 fail** |
| apps/mobile 全量 | `pnpm --filter @weknora/mobile test` | **164 pass / 0 fail / 6 skip**（skip 为既有 opt-in 真实 HTTP 集成用例，本地无部署环境如实跳过） |
| 类型检查 | `pnpm --filter @weknora/mobile typecheck` | **通过（tsc --noEmit 无输出）** |

注：`go test -v` 管道计数命令以 `-count=1` 实跑三次（plain/v-count/fail-count），TS 各命令均在本 worktree 根实跑，无任何替跑或缩量。

## 残留核对

- `grep -n "19 个" docs/plans/issue30-sweep/plans/plan-t56.md` → 无残留。
- `grep -n "replay?: boolean|replay === true|replay: true"` 在 plan-t56.md 与 voice.ts → 无残留；`replay` 仅存于 voice.ts:31 的移除说明注释与 voice.test.ts:94-102 的回归测试（有意保留的钉子）。
