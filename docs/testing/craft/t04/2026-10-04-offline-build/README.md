# T04（#123）固定离线运行时构建验收 — 2026-10-04 复验轮

- 分支：`chore/craft-t04-close` @ `d5718d2a8`（worktree `/tmp/wk-t04`）
- 镜像：`weknora-craft-runtime:t04-web`，image ID `sha256:4ec3a6a0f720c571b0b506e233bd603bbfaa882a4fccc29a65d1439cec8e0873`
- 前置事实：`docker/craft/web/`（build.py / toolchain.lock.json / runtime-config.json）在 71809b474（2026-09-29）有变更，晚于旧轮镜像（09-26 `6b924da64171`），故本轮按 Dockerfile lineage 重建镜像后再采集证据。

## Pinned digests（三方一致：build-log.json == web/toolchain.lock.json == runtime-config.json）

| 项 | 值 |
| --- | --- |
| image ID | `sha256:4ec3a6a0f720c571b0b506e233bd603bbfaa882a4fccc29a65d1439cec8e0873` |
| toolchain_digest | `98e903b2383b522c66c733c75e092a2f62d52d6fc107bf73d1111ca87d593195`（09-29 变更后新 pin；旧轮为 `5d9d7d28…`，镜像陈旧即本轮重建的原因） |
| template_sha256 | `ecf1a109e97e0d991aeb4507fdb5079d71576f2caff1608491bf05961ae16d61`（template v1.0.0，未变） |
| 镜像内逐文件 pin 校验 | Dockerfile W1 层构建时 sha256sum -c 四件全过 + `--selftest` 离线冒烟过（见 docker-build-image.txt） |

## 1. 镜像构建（docker-build-image.txt）

```
DOCKER_BUILDKIT=0 docker build --build-arg TARGETARCH=arm64 \
  -t weknora-craft-runtime:t04-web -f docker/craft/Dockerfile docker/craft
```

- legacy builder 不注入 `TARGETARCH`，须显式 `--build-arg TARGETARCH=arm64`（首次无该参数失败于 Step 9，已如实记录于 log）。
- exit 0 → `Successfully built 4ec3a6a0f720`。

## 2. 真实离线沙箱构建（docker-offline-build.txt，stdout/stderr 为空即证据之一）

```
docker run --rm --network none --read-only \
  -u 10001:10001 --entrypoint /usr/bin/python3 \
  -e HOME=/tmp/t04-empty-home -e TMPDIR=/tmp/t04-empty-tmp \
  --tmpfs /tmp:rw,size=64m,uid=10001,gid=10001 \
  -v <input>:/workspace/input:ro -v <output>:/workspace/output:rw \
  weknora-craft-runtime:t04-web \
  /opt/craft/web/build.py --toolchain /opt/craft/web \
    --input /workspace/input --output /workspace/output \
    --runtime-digest sha256:4ec3a6a0f720c571b0b506e233bd603bbfaa882a4fccc29a65d1439cec8e0873
```

结果：**exit 0**，无任何输出；产物恰为 entry + 本地资产 + 构建日志四件（`offline-build-output/`）：

```
index.html            # 仅本地相对引用 assets/craft-web.css、assets/craft-web.js
assets/craft-web.css
assets/craft-web.js
build-log.json        # runtime_digest=镜像真实 ID, toolchain_digest=新 pin, exit_code=0, egress=denied
```

输入为暂存的 `content.json`（华东区销售网页样例：title/subtitle/lang + 两个 section——table 与 html 各一）。无任何下载发生；HOME/TMPDIR 指向 tmpfs 空目录，不触宿主缓存。

## 3. 拒绝出网探针（denied-egress-probes.txt）

同一镜像、`--network none --read-only -u 10001:10001` 逐通道探测，五路全部非零退出（拒绝）：

| 通道 | 退出码 | 拒绝形态 |
| --- | --- | --- |
| HTTP（urllib） | 1 | URLError: Temporary failure in name resolution |
| DNS（getaddrinfo） | 1 | gaierror |
| Git（ls-remote） | 1 | 镜像无 git 二进制 → 无法发起（结构性 fail-closed） |
| 包管理器（pip install） | 1 | 真实触达网络层后 DNS 拒绝 |
| shell（curl https） | 6 | Could not resolve host |

`--network none` 下容器无网络设备，出网结构性不可能；探针为该不变量的直接演示。

## 4. 验证命令（branch @ d5718d2a8）

```
go test ./internal/container/... -run Craft -count=1
  ok  github.com/Tencent/WeKnora/internal/container          35.668s
  ok  github.com/Tencent/WeKnora/internal/container/bootsmoke 4.540s [no tests to run]

go test ./internal/modules/craft/... -run 'Release|Web' -count=1
  ok  github.com/Tencent/WeKnora/internal/modules/craft      0.323s
```

（含 `TestCraftWebToolchainPinsMatchShippedFiles`：真实文件字节 ↔ toolchain.lock.json ↔ runtime-config.json 三方机器校验。）

## 文件清单（SHA256SUMS）

`docker-build-image.txt` / `docker-offline-build.txt` / `denied-egress-probes.txt` / `offline-build-output/{index.html,build-log.json,assets/craft-web.css,assets/craft-web.js}`
