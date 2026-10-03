# 2026-10-03 全范围 OCR 门禁重试记录（第 6 次尝试）

## 结果：partial→complete——第 7/8 次 resume 补全后门禁通过

- 会话：`0f0bcb06-bbfe-4d2c-b330-5ad829c6a0ba`（2026-10-03 16:06 启动，17:45 退出，约 99 分钟）
- 范围：`db234c5eb171f2dde7427d382b55b503a038f879..b3d48d5cb9cd8833ea348d4a6f37a9bef52ae13b`——addendum/OCR 修复计划载明的既定 review target；两端均已是 main 祖先（经 sweep-integration 合并 `34d1412ee`），**未使用替代范围**。
- 命令：`ocr review --from db234c5eb… --to b3d48d5cb… --audience agent --background "…" --output …`；启动前 `ocr llm test` 连接与工具往返均通过。
- 覆盖：333 selected，**82 failed**（251/333 ≈ 75%）；失败原因＝provider HTTP 429 code 1302（账户速率限制，重试 6 轮后放弃）及少量超时/取消（retry report：95/1819 请求受影响，30 失败 11 取消 54 恢复）。
- 产出：378 findings（**0 critical / 13 high / 120 medium / 245 low**），首次产出完整报告文件。判据口径＝「selected 全覆盖且零 LLM 失败」→ 当时未过门。

## 第 7/8 次尝试（2026-10-03 晚，配额窗口恢复后）：COMPLETE，门禁关闭

7. **`617ad6a5`（20:15-21:27，1h11m）**：`--resume 0f0bcb06…` + `--concurrency 4`（降并发避 429）。251 项复用 + 82 项重跑，81 done / 1 failed（wiki-reader.css，main_task 15 分钟超时，round 2/2），全程**零 429**。产出 510 findings 部分报告 `ocr-2026-10-03-fullrange-resumed.md`。
8. **`786220f3`（21:27-21:35，7m19s）**：对上一会话再 `--resume` + `--concurrency 2 --timeout 25`，332 项复用 + 唯一失败项重跑成功。**333/333 全覆盖、零失败，511 findings**，完整报告 `ocr-2026-10-03-fullrange-final.md`。

- 增量 133 条（5H/35M/93L）全落在此前未覆盖的 56 文件；裁定见 `ocr-2026-10-03-fullrange-final-increment.md`：**open-high 4 / open-medium 27 / needs-ruling 3 / fixed-on-main 或 FP 6**（含 H-A5 已修）。
- 工程注：ocr session 按 cwd 键控，从 /tmp/wk-ocr resume 需把会话 jsonl 复制到 `~/.opencodereview/sessions/private-tmp-wk-ocr/`（macOS /tmp 实路径）。

## 历史尝试（累计 6 次未完成）

1. 2026-09-29 最终命令会话：333 selected 全失败（429），零报告。
2. 2026-09-29/30 全量重试：333 项中断于 429/1308 五小时配额，零完成。
3. 2026-09-30 会话 `c3984cc3`：216/333 完成、117 失败、18 LLM 失败；resume 因规则身份不一致被拒。
4. 2026-09-30 会话 `b5b5cf36`：编排中止（15 comments 部分收获，即 4 medium+11 low 的来源）。
5. 2026-09-30 会话 `b3e6486a`：0 覆盖中止。
6. **本次（2026-10-03）`0f0bcb06`：历史最深——75% 覆盖、378 findings、首次产出报告文件，仍败于同源 429。**

## 后续处置

- **High/Medium 133 条**：单独报裁（digest：`ocr-2026-10-03-high-medium-digest.md`），本轮零内联修复（核销轮约束：生产码零改动）。
- **Low 245 条**：因报告非全量（82 项无覆盖），不对不完整快照做逐条终局裁定，待门禁通过后一并处理。
- **重试**：配额窗口恢复后优先 `ocr review --resume 0f0bcb06-bbfe-4d2c-b330-5ad829c6a0ba`；若按修复后 HEAD 收口则改用 `--to 1f7ca8519a384bbc6bff554e27462ab6a276f832`（b3d48 之后还有 `6df56a011`/`1db12dcca`/`1f7ca8519` 三个修复提交，本次报告中的 wx-driver 凭据、embed 样式、files.ts 401 分类发现在 main 上已修复，裁定时须对 main 现码复核）。
