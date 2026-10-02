# T14 SOCK_DIAG 活体验收复跑（2026-10-02，Task 2 重写轮）

复现 2026-09-29 轮（`../2026-09-29-sockdiag-live-acceptance/`）的七判据活体验收。
差异：宿主侧 `controller.py` 与 `barrier_adapter.py` 为不可恢复后的**重写件**
（原哈希 `9e684035…`/`598502bb…`，见 `deploy/craft/render-boundary/policy-helper/RECOVERY.md`）；
helper.py 与 tests/test_controller_integration.py 为 Task 1 恢复件，哈希与 09-29 轮
source_checkpoint 完全一致，测试文件未被改动（冻结契约）。

## 结果

七判据全部通过（`live-test-run.json`，14.3s vs 09-29 轮 158.4s）：

| 判据 | 结果 | 证据 |
| --- | --- | --- |
| same-fd 复用 marker 双向 ACK 例外计数器正增量 | PASS（两侧 +3，handles 一致） | `…/webdriver-ack-marker-proof.json` |
| 同源端口重绑后新 SYN 被拒（TimeoutError 502ms，loopback drop +1，ACK 例外 0/0） | PASS | `…/same-port-syn-proof.json` |
| 异源端口 fresh SYN 被拒 | PASS | `policy.json → mandatory_canary.webdriver_same_listener_canary` |
| loopback 服务（8081）被拒并计数 | PASS | `mandatory-canary.json` |
| 随机 preview 端口可用 | PASS（58005） | `preview-port-attestation.json`、测试体 |
| 强制 drop-counter 金丝雀 | PASS | `mandatory-canary.json` |
| target policy receipts（4 目标 + dns_invalid 隔离计数） | PASS | `…/target-probe-events.json` |

清理验证（`live-cleanup-verification.json`）：helper label 过滤空、renderer 已销毁、
策略名容器无残留；计数器金丝雀终值冻结于 same-port 证明，renderer 网络命名空间
随销毁消失，事后不可再读即隔离性证明。

## 与 09-29 轮对比

- 证据结构一致：`.controller.lock`/`.sequence` + `NNNN-<action>.json` 序列记录
  （baseline→install→mandatory-canary-before/after→webdriver-canary-after→snapshot×16）
  + 命名证明文件；raw_refs（nft-rules/routes/links/container-inspect）逐条带 sha256。
- 流程一致：flow = 127.0.0.1:随机口→9515 ESTABLISHED，helper 内 SOCK_DIAG attestation
  + 两条 `tcp flags & (syn | ack) == ack` 精确四元组例外规则。
- 环境漂移：本机 registry mirror 对锚定 base digest 的 manifest HEAD 返回 403，
  故 `DOCKER_BUILDKIT=0` + Dockerfile 引用本地 tag（本地镜像 ID == 锚定 digest
  `4c47124a…`，digest 形式保留在 build.sh）。
- 09-29 轮的 controller.py/barrier_adapter.py 哈希不再适用；重写件哈希见
  `live-test-run.json → source_checkpoint`。
