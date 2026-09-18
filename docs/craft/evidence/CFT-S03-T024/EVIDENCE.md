# CFT-S03-T024 里程碑证据 —— 网页纵向闭环与反例

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交
**这是首个产品里程碑（CFT 任务图的 T024）。**

## 一、主路径（创建 → … → 重开恢复）——两种模式真实浏览器执行

### mock 模式（确定性 fixture 子模型，全量 6 spec）

```
bash e2e/craft-stack.sh up mock && run mock   →  6 passed (27.8s)，exit 0
```

01 spec 即完整主路径：创建（POST /craft/sessions 201）→ 上传并关联（sha256 校验 + resource:// 引用）→ 主 Run（tRPC 入场，SSE）→ OpenCode 委派（craft_delegate，5/5 succeeded）→ 校验发布 v1 → 独立 https 预览源（真实 iframe、按月报告、筛选交互）→ 修改提交发布 v2 → v1/v2 下载 SHA 不变/不同 → 02 spec 重开恢复（状态/两版本/当前版本选择全恢复）。（mock-full-suite.log）

### real 模式（真实 OpenCode 1.18.4 内置免费模型，无任何用户凭据）

```
bash e2e/craft-stack.sh up real && run real --grep "01 create|02 reopen|04 cross"
  →  3 passed (2.1m)，exit 0    （real-mode-suite.log；截图 real-v1-preview/real-v2-versions/real-cross-tenant.png）
```

真实模型读到落盘的 craft-sales.csv，两轮真实生成（月度 v1 + 季度 v2）、逐轮发布、预览、历史下载、重开恢复、跨租户不可见——W06 的 real 证据在本轮 HEAD 复跑成立。

### 执行环境

OpenCode 1.18.4 sha256 `9449af91…f98`（与 docker/craft/runtime-config.json 一致）；全栈 harness（craft-stack.sh）：Go server（WEKNORA_CRAFT_ENABLED=true + recovery worker + kinds=web）、SQLite 全迁移链、本地对象存储、nginx 独立 https 预览源（自签）、vite dev。测试凭据仅存 /tmp 运行目录（0600），不入库。

## 二、十项反例 —— 每项映射到本轮/历史实际执行的证据

| # | 反例 | 证据（均为本轮或既有已执行测试） |
|---|---|---|
| 1 | 提交响应丢失 | e2e 06（刷新生成中页面 DB 计数恰 1 次 admission）+ T008 命令桥（重试同 requestId payload deepEqual）+ Go `craft_session_test`（同 key 重放原 run） |
| 2 | 断流与重复 seq | e2e 05（切会话旧流零追加，mock+real 均跑）+ T007 replay 套件（同帧双重放消息/ID/正文零变化）+ `core/controller.test.ts` 断流恢复（W04） |
| 3 | 游标缺口 | `domain/craft/reconnect.test.ts`（缺口/游标过期/新 generation 重读权威快照——W04 语义） |
| 4 | 旧 worker | T016 `GuardStaleFenceCannotPublish`（epoch 提升后 plan"lease lost"/prepare Conflict/SaveResult 拒绝/零版本）+ T014 重启读回 |
| 5 | 审批送达不明 | T023 既有 C02 套件：delivery unknown 不自动扫、`SIGKILL` 双窗口（决定已存/OC 已接受未 ack）重启后恰一次送达零重复 + T012 UI"送达不明，需核对" |
| 6 | 取消竞态 | T023 `StopStatus` 五态矩阵（受理≠终止，aborted+idle 双证才 canceled）+ 终态交互迟到批准 ErrGone(410) 不转发 |
| 7 | 跨租户访问 | e2e 04（mock+real 均跑：跨租户 404、viewer 写 403）+ T017 source guard（跨租户知识 Forbidden 零落盘） |
| 8 | 来源撤权 | T017（撤权后下一次 Build 即 Forbidden、无陈旧权限快照、零新增写入） |
| 9 | 预算触顶 | T018（拒绝时零预留零调用记录）+ 既有 `LastQuotaConcurrentAuthorizationAdmitsExactlyOne`（并发末额恰一） |
| 10 | 工作区恢复冲突 | T022 既有 `RestoreRefusals`（活动 run ErrBusy / revision 竞态 ErrConflict）+ HTTP `RestoreCompetitions` |

## 三、环境事件与修复记录（本轮）

1. Playwright headless-shell 缓存被外部清理（ms-playwright 目录仅剩空壳）→ 经 npmmirror 镜像重装（`PLAYWRIGHT_DOWNLOAD_HOST=cdn.npmmirror.com/binaries/playwright`）。
2. 一次 mock 失败轮的 stack 残留进程占用固定端口 → `down mock` 清理后 real 正常启动。
3. **nginx 二进制被外部清理消失**（旧 master 进程仍活但磁盘无二进制）→ `brew install nginx`（1.31.6）恢复。
以上均为本机测试环境修复，未触碰任何生产/用户数据。

## 四、未验证事项

- real 模式的 03/05/06 spec（UI 语义断言）本轮未重复执行——W06 记录该决策（真实模型轮时长 1-2 分钟/轮，UI 语义由 mock 全量覆盖）；如需可在 T035 总回归一并跑。
- 十反例中 #3（游标缺口）的浏览器级注入未单独做（domain/controller 层 pin + e2e 05 的行为级覆盖）。

## 五、结论

**T024 首个产品里程碑达成**：网页类型在 mock 与 real 两种模式下、真实浏览器、真实全栈（含真实 OpenCode 委派与独立预览源）完成完整闭环与重开恢复；十项反例全部有已执行的测试证据（本轮新增 pin + 既有套件 + e2e），无一项以"计划"或"未跑"充数。
