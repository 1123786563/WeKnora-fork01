# T04（#123）固定离线 Web 构建运行时 —— 环境级验证证据

- 日期：2026-09-25
- Lane worktree：`/Users/wuyongjun/.codex/worktrees/craft-107-t04/WeKnora-fork01`（分支 `codex/craft-107-t04`，基点 = 集成分支 HEAD `25caebd13`）
- 镜像：`weknora-craft-runtime:t04-web`，本地镜像 ID `sha256:57ae969fe89f16b5ed8a0de82cb3f83ca79fc0ca3632e654354b2c8358899f85`（由 `docker build -f docker/craft/Dockerfile docker/craft` 产出，W1 web 工具链层的逐文件 sha256 校验与 `--selftest` 离线冒烟在镜像构建内执行，任一失败都会使构建失败；本构建 exit 0）
- 验证对应 Issue #123 验证证据第三条：“Provide a real offline sandbox build log and a denied-egress probe”。

## 1. 真实离线沙箱构建（clean sandbox，无下载、无宿主缓存/HOME）

命令（宿主执行）：

```
docker run --rm --network none --read-only \
  -u 10001:10001 --entrypoint /usr/bin/python3 \
  -e HOME=/tmp/t04-empty-home -e TMPDIR=/tmp/t04-empty-tmp \
  --tmpfs /tmp:rw,size=64m,uid=10001,gid=10001 \
  -v <input>:/workspace/input:ro -v <output>:/workspace/output:rw \
  weknora-craft-runtime:t04-web \
  /opt/craft/web/build.py --toolchain /opt/craft/web \
    --input /workspace/input --output /workspace/output \
    --runtime-digest "sha256:t04-offline-probe-1790338831"
```

要点：

- `--network none`：容器无任何网络设备（出网结构性不可能）；
- `--read-only` 根文件系统 + 只挂 `/workspace/output` 可写、`/workspace/input` 只读；
- 非根运行用户 10001:10001（镜像 craft 用户）；HOME/TMPDIR 指向 tmpfs 空目录——不使用任何宿主缓存或宿主 HOME；
- 构建输入为暂存的 `content.json`（区域销售网页样例）。

结果：**exit 0**，stdout/stderr 为空（`docker-offline-build.log`），产出仅：

```
output/index.html
output/assets/craft-web.css
output/assets/craft-web.js
output/build-log.json
```

`build-log.json`（`offline-build-output/build-log.json`，未截断原文）：

```json
{"assets": ["assets/craft-web.css", "assets/craft-web.js"], "egress": "denied", "entry": "index.html", "error": "", "exit_code": 0, "kind": "web", "runtime_digest": "sha256:t04-offline-probe-1790338831", "schema": 1, "template_sha256": "ecf1a109e97e0d991aeb4507fdb5079d71576f2caff1608491bf05961ae16d61", "template_version": "1.0.0", "toolchain_digest": "9459a7f47c76b440e1f5594c4836de856f70e7bc92cb1e3fb946736ab0ca705f"}
```

日志标明模板/运行时摘要与真实退出码（0）；入口与资产全部本地相对引用（`offline-build-output/index.html` 可复核：仅 `assets/craft-web.css`、`assets/craft-web.js`）。

## 2. 拒绝出网探针（denied-egress probes）

同一镜像、同一 `--network none` 形态逐路探测（完整记录见 `denied-egress-probes.log`）：

| 通道 | 命令 | 退出码 | 结果 |
| --- | --- | --- | --- |
| HTTP | `python3 -c "urllib.request.urlopen('http://example.com',timeout=5)"` | 1 | 拒绝（URLError） |
| DNS | `python3 -c "socket.getaddrinfo('example.com',443)"` | 1 | 拒绝（gaierror） |
| Git | `git ls-remote https://github.com/...` | 127 | 拒绝（镜像不含 git 二进制：Git 网络尝试无法发起，fail-closed） |
| 包管理器 | `python3 -m pip install --no-input --no-cache-dir --break-system-packages --timeout 5 markdown` | 1 | 拒绝（`Temporary failure in name resolution`，真实触达网络层后被 DNS 拒绝） |
| shell | `/usr/bin/curl -fsS --max-time 5 https://example.com` | 6 | 拒绝（`Could not resolve host`） |

五路全部非零退出（拒绝）。其中 Git 一路为"二进制未预置→无法发起"的结构性拒绝，与“构建不依赖任何下载”的镜像设计一致，如实记录。

## 3. 复现

```
docker build -t weknora-craft-runtime:t04-web -f docker/craft/Dockerfile docker/craft
# 离线构建与探针命令见上（--network none --read-only，非根 10001，空 HOME/TMPDIR）
```

摘要链一致性由 `go test ./internal/container -run 'TestCraftWebToolchainPinsMatchShippedFiles'` 机器校验（真实文件字节 ↔ toolchain.lock.json ↔ runtime-config.json web_toolchain 节三方一致，Go 与 Python 派生逐字节相同）。
