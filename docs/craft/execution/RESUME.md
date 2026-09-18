# CFT 续跑入口（RESUME）

最后更新：2026-09-18（恢复轮 —— 36/36 verified 结论不变；台账/证据被误删后已恢复，T027/T031 证据重生成，主线迁移撞号已修）

## 最终状态

- **36 项任务全部 verified（自审）**，每项带独立证据目录：`docs/craft/evidence/index.md` 为总索引。
- 原 `craft/cft-execution` 分支（42 个本地提交，未 push）已经由合并提交 b08a67a6 全量并入 **main**；该分支与 `.worktrees/craft-cft` worktree 已删除，续跑一律在主仓 main 进行。
- 首个产品里程碑 CFT-S03-T024 双模式达标（mock 6/6 + real 3/3 真实 OpenCode 免费模型）。
- 恢复轮要点（DECISIONS D004/D005）：a35b0389 误删的 84 个台账/证据文件已由 cbb60257 恢复；T027/T031 证据在 main 真实重跑重生成；`migrations/sqlite` 三 lane 撞号 000058 已重编号修复（paseo→000077、execution_registrations→000078），修复后 Craft 后端四包 `-run TestCraft` **119 PASS / 0 FAIL**。

## 后续会话入口（若继续）

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01   # 主仓 main（craft-cft worktree 已删，勿再找）
# 复验：pnpm run test:craft:shared（108）| go test ./internal/craft/... ./internal/application/service/ ./internal/handler/session/... ./internal/agent/opencode/... -run TestCraft
# 未验证项（见 docs/craft/evidence/index.md §未验证项）：
# 1) real 模式 03/05/06 spec；2) 生产 release-evidence 重生成 + 灰度演练；
# 3) Mimosa 219 存量安全债治理；4) 逐像素高保真 diff；5) 暗色主题（边界外）
```

## 环境坑（累计）

1. e2e 前检查 `nginx -v` 与 playwright 缓存（本机多起外部清理；playwright 用 npmmirror 镜像重装 headless-shell）。
2. **PATH 含带空格目录（"Application Support/…"）会断 `env $OC_ENV` 分词 → serve 起不来**：`export PATH=$(echo $PATH | tr ':' '\n' | grep -v " " | paste -sd: -)` 后再 up。
3. 超时 panic 会遗留 serve 进程占端口：`down` 清理后再 up。
4. live 测试残留进程占 XDG → 杀干净再跑 CRAFT_LIVE。
5. **并行会话清理提交可能误删台账**（a35b0389 前科）：每轮开始先 `git ls docs/craft/execution | head` 验在位，缺失则从 b08a67a6 恢复（恢复法见 DECISIONS D004）。
6. 多 lane 并行加迁移必须先查 `migrations/sqlite` 尾号是否被占（000058 撞号前科，D005）。

## 未提交改动 / 进程

- 恢复轮以两个提交收口（迁移重编号 + 证据重生成/台账更新），提交后工作树干净；全部提交仅在本地 main，未 push、无外部 PR、未部署。
