# CFT 续跑入口（RESUME）

最后更新：2026-09-18（T036 完成 —— **36/36 全量 verified，任务图收官**）

## 最终状态

- **36 项任务全部 verified（自审）**，每项带独立证据目录：`docs/craft/evidence/index.md` 为总索引。
- 分支 `craft/cft-execution` 本地提交 42 个（未 push、无外部 PR、未部署）。
- 首个产品里程碑 CFT-S03-T024 双模式达标（mock 6/6 + real 3/3 真实 OpenCode 免费模型）。

## 后续会话入口（若继续）

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/craft-cft
# 复验：pnpm run test:craft:shared（108）| go test ./internal/... | e2e mock 6 spec
# 未验证项（见 docs/craft/evidence/index.md §未验证项）：
# 1) real 模式 03/05/06 spec；2) 生产 release-evidence 重生成 + 灰度演练；
# 3) Mimosa 219 存量安全债治理；4) 逐像素高保真 diff；5) 暗色主题（边界外）
```

## 环境坑（累计）

1. e2e 前检查 `nginx -v` 与 playwright 缓存（本机多起外部清理；playwright 用 npmmirror 镜像重装 headless-shell）。
2. **PATH 含带空格目录（"Application Support/…"）会断 `env $OC_ENV` 分词 → serve 起不来**：`export PATH=$(echo $PATH | tr ':' '\n' | grep -v " " | paste -sd: -)` 后再 up。
3. 超时 panic 会遗留 serve 进程占端口：`down` 清理后再 up。
4. live 测试残留进程占 XDG → 杀干净再跑 CRAFT_LIVE。

## 未提交改动 / 进程

- 见最终提交（本文件随 T036 一并提交后工作树干净）；无活跃 stack/进程；主工作区（main）其他会话改动未触碰。
