# apps/mobile 来源说明（Vendor Snapshot）

本目录是 [Conduit](https://github.com/cogwheel0/conduit)（Open WebUI 移动端 +
多后端直连客户端，Flutter）的**单提交快照**，作为 WeKnora 的移动端引入，含
一等公民的 WeKnora Direct 适配器。

| 项 | 值 |
|---|---|
| 源仓库 | https://github.com/cogwheel0/conduit |
| 源分支 | `feat/weknora-adapter` |
| 快照提交 | `d64154c` Add WeKnora direct provider adapter（2026-09-19） |
| 上游基线 | `57da918`（上游 main 当时位置） |
| 引入时的本地改动 | 仅 WeKnora 适配器相关 21 个文件（详见 `docs/WEKNORA.md`） |

## 与源仓库的差异（快照处理）

- **剔除** `.git/`、`node_modules/`、`.dart_tool/`、`build/`、`.mimosa/`。
- **剔除参考用 submodule**：`openwebui-src/`、`hermes-src/`（上游仅作 API
  参考，不参与构建，见上游 `docs/BUILDING.md`）。
- **内联（去 submodule 化）**：`third_party/mermaid/`、`third_party/katex/`
  的内容直接拷入（已剥离内层 `.git` 指针）。这两个是 `pubspec.yaml` 的 path
  依赖，上游以 submodule 提供；快照内联后无需 `git submodule update`。
- **删除** 上游 `.gitmodules`（四条 submodule 记录在快照中全部失效）。

因此本目录**不需要** submodule 初始化，Flutter 侧从：

```bash
cd apps/mobile
flutter pub get
dart run build_runner build --delete-conflicting-outputs   # 生成 riverpod/l10n 代码
flutter analyze
flutter test
```

即可工作（iOS/macOS 构建还需 Xcode，Android 需 Android SDK）。

## 后续同步上游

快照不带 conduit 历史，同步 = 重放差异：

```bash
# 在 conduit fork 里跟进上游后，对比 feat/weknora-adapter 与本目录
diff -qr --exclude=.git --exclude=node_modules --exclude=.dart_tool \
  --exclude=build --exclude=.mimosa \
  /path/to/conduit apps/mobile
```

约定：**上游同步改动在本目录内落地时，同步更新本表"快照提交"行**，并在
commit message 里同时给出上游 commit 区间。

## 许可证

沿用上游 `LICENSE`、`PRIVACY_POLICY.md`、`THIRD_PARTY_NOTICES.md`（随快照
保留在本目录内），未做任何删改。
