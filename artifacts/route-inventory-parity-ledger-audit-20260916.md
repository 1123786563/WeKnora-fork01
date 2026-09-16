# Route inventory 与整体 parity 台账审计（2026-09-16）

## 范围与结论

本审计仅检查 `docs/migrations/react` 与 `artifacts`；没有修改 Web、Embed、Desktop、Vue、Go 或移动端业务代码。审计对象是当前工作树：

`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`

结论：路由清单已经覆盖主要 SPA、历史兼容入口、Settings section、特殊文件/WS/Embed 入口，但当前文档存在可复核的口径漂移和 authority 追溯缺口。因此本审计不把整体 parity 标记为 accepted。

## 可复核的当前快照

执行命令：

```text
wc -l docs/migrations/react/route-parity.csv
python3 - <<'PY'
import csv, pathlib, collections
root = pathlib.Path('.')
rows = list(csv.DictReader((root/'docs/migrations/react/route-parity.csv').open()))
print('rows=', len(rows))
print('kinds=', dict(collections.Counter(r['kind'] for r in rows)))
print('duplicate_routes=', [k for k,v in collections.Counter(r['route_or_entry'] for r in rows).items() if v > 1])
print('explicit_row_id=', 'row_id' in rows[0])
for r in rows:
    p = r['current_source']
    if p and ';' not in p and not (root/p).exists():
        print('missing_source=', r['route_or_entry'], p)
PY
```

结果：

| 检查 | 当前结果 | 证据层 |
|---|---:|---|
| `route-parity.csv` 数据行 | 59（含表头共 60 行） | static |
| kind 分布 | page 18、settings-section 26、redirect 5、layout+redirect 1、dev-only 1、independent-entry 1、special-file-route 5、special-websocket 1、special-embed-route 1 | static |
| `route_or_entry` 重复 | 0 | static |
| `current_source` 不存在 | 5 条 | static |
| 其中 `/platform/apps*` Vue 源码路径不存在 | 4 条 | static |
| `embed.html` 组合路径作为单一文件检查失败 | 1 条 | static；需拆分检查 |

缺失路径的具体值：

```text
/platform/apps                 frontend/src/views/apps/AppsView.vue
/platform/apps/connections     frontend/src/views/apps/ConnectionsView.vue
/platform/apps/authorization/:id frontend/src/views/apps/AuthorizationView.vue
/platform/apps/actions/:id     frontend/src/views/apps/ActionView.vue
embed.html                     frontend/embed.html; frontend/src/embed-main.ts
```

这不证明 React Apps 实现不存在；当前 React 实现位于 `apps/web/src/apps/AppsPages.tsx` 等文件。四个 Apps Vue authority 已在历史提交 `9b0c11c4` 解析，映射与复核命令见 `artifacts/apps-vue-authority-20260916.md`。`embed.html` 仍应拆成两个可分别存在性检查的路径。

## 发现

### P1 — 当前行数与 canonical matrix/baseline 口径不一致

- `route-parity.csv` 当前为 59 条数据行。
- `docs/migrations/react/vue-react-parity-matrix.md:17` 仍写 `route-parity.csv (53 rows)`。
- `docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md:50,60,84,101` 仍写 53 行/`R001-R053`，同时同一文件的校验记录又写 `R001-R056` 和 56 个 R 行。
- `artifacts/parity-agent-inventory.md:24,32` 仍把 CSV 描述为 53 行。
- `artifacts/independent-acceptance-20260915.md:11` 已写当前为 59 行，说明不是单纯的未来计划数字。

影响：读者无法从 matrix、baseline、inventory、acceptance 得出唯一的当前 route inventory 规模；“全部覆盖”声明的分母不稳定。历史 baseline 可以保留，但必须显式标为 historical snapshot，并在 canonical matrix 顶部给出 current snapshot=59。

### P1 — CSV 没有稳定 row ID 列，却被其他文档当作稳定行 ID 清单

`docs/migrations/react/route-parity.csv:1` 的字段从 `route_or_entry` 开始，没有 `row_id`；而 `docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md:84,101` 声明每行有稳定 `R001-R053`，`docs/migrations/react/vue-react-parity-progress.md:1057,1066` 又以 R/N 行数作为 sanity check。

影响：CSV 的插入、排序或删除会改变隐式行号，无法独立复核 matrix/ledger 的 R 编号映射。应在文档范围内补充显式 ID 列或提供机器可读的 route-to-row 映射；在此之前，不能把“按 CSV 顺序对应”当成稳定 contract。

### P1 — Apps Vue authority 已解析，运行时 parity 仍未闭环

CSV 的四个 `/platform/apps*` 行在当前 checkout 中没有物理文件，但历史提交 `9b0c11c4` 提供了可复核的 Vue authority；现有 React Apps 实现与该历史基线的视觉、权限和 mutation 对照仍需独立验收。

影响：Apps 的“Vue 基线”目前只能证明为未解析的来源引用，不能支撑视觉、交互、权限和生命周期 parity。应在 authority 解析完成前将这四行标为 `source_status=unresolved` 或 `react-only/compatibility`，并保留待确认项。

### P2 — 证据引用存在通配符/非文件路径，机器存在性检查会产生假阳性

对 ledger 中 `artifacts/...` 引用做路径检查时，以下值不是可直接打开的文件路径：`artifacts/parity-agent-*`、`artifacts/references/`。它们来自说明文字/通配符，不能作为证据链接。另有 `artifacts/browser-runtime-evidence-20260915.md` 被引用但当前不存在，应在引用处改成实际文件或明确 `blocked/missing`。

### P2 — 早期 evidence 的验证命令与当前工作树不应混作 current acceptance

`docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md:101-105`、`artifacts/independent-acceptance-20260915.md:45-58` 记录了不同日期、不同测试计数和不同工作树状态。它们可以作为历史证据，但不能覆盖 2026-09-16 当前 SHA 的全量 route/state/browser 证据。`docs/migrations/react/vue-react-parity-progress.md:3709-3733` 的 R397 已正确把最新检查限定为 static/unit/build 与匿名 browser/backend，且保留认证、真实 mutation、Embed 宿主和 Wails 缺口；该边界应作为当前 ledger 的主口径。

## 证据层结论

| 要求 | 当前证据 | 结论 |
|---|---|---|
| route/source 静态盘点 | CSV、route inventory、matrix、历史 Apps authority artifact | Apps authority 已解析；row-ID contract 未闭环 |
| 单元/类型/构建 | R397 ledger 与相关 artifacts | 只能证明实现/测试层，不证明逐页 parity |
| mock/fixture | 各模块 evidence 中的 fixture 记录 | 只能证明测试分支，不等于真实后端 |
| 匿名浏览器 | `artifacts/authenticated-browser-backend-audit-20260916.md` 及 `browser-evidence-20260916-r2` | 只证明登录、匿名保护路由和 401 边界 |
| 认证浏览器/真实权限 mutation | 当前无安全凭据 | `blocked-env` |
| Wails 原生逐功能 | 当前无完整启动/OS 交互闭环 | `blocked-env` |
| 全量视觉/交互逐页 acceptance | 没有同条件全页面 computed-style、截图、交互和后端闭环 | 未通过 |

## 建议的文档修复顺序

1. 在 canonical matrix 顶部增加 current inventory snapshot（59 rows、kind counts、SHA/date），并把 53/56 数字标为历史快照。
2. 给 `route-parity.csv` 增加显式稳定 ID，或新增同目录映射文件并让 sanity check 校验它。
3. 保留四个 Apps Vue authority 的历史提交映射；在视觉、权限和 mutation 验收前不要把 React Apps route 标成 parity accepted。
4. 清理通配符/目录型 artifact 引用，并将缺失证据明确标注为 `missing`/`blocked-env`。
5. 保持 R397 的证据分层和未完成结论，待认证凭据、浏览器策略、Embed token 与 Wails 环境恢复后再做最终 acceptance。
