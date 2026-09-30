# T04：固定版本 Artifact 授权下载

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

已授权用户以固定产物身份取得短时下载授权，旧链接在撤销或删除后失效，为求职材料版本提供通用安全能力。

## Acceptance criteria

- [ ] 授权绑定 Tenant、所有者、资源及确定版本
- [ ] 下载时重新检查授权和撤销状态，不仅在签发时检查
- [ ] 跨 Tenant、过期、篡改签名和已删除版本被拒绝
- [ ] 现有 Task 产物下载继续可用

## Blocked by

None (can start immediately)。

## Ownership and interfaces

- 主责边界：Workbench Artifact；现有 Task 产物路径。
- 消费接口：现有 Workbench Artifact 授权与存储权威。
- 交付接口：确定版本的短时下载授权、撤销后下载拒绝合同。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Workbench 公共接口测试检查成功、越权、过期、撤销。
- 使用一个真实产物从 Web 下载并比对内容摘要。

## Failure and unknown results

签发结果未知不得伪造可用链接；拒绝响应不泄露资源是否存在。

## Parallel scheduling

- 理论前沿：G0；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
