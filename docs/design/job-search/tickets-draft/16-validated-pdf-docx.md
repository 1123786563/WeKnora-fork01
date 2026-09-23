# T16：同版 PDF/DOCX 生成与验证

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户从一份已确认材料版本取得内容一致的 PDF 和可编辑 DOCX。

## Acceptance criteria

- [ ] 两种文件绑定同一正文摘要与材料版本
- [ ] PDF 校验可提取文本与页面版面，DOCX 校验完整与可编辑性
- [ ] 两种验证均通过后版本才可标记可用于投递
- [ ] 旧版本保持可下载，删除或撤销后旧授权立即失效

## Blocked by

T04、T15。

## Ownership and interfaces

- 主责边界：Career Material Export、Workbench Artifact；Web 文件获取。
- 消费接口：已确认材料版本与固定 Artifact 授权。
- 交付接口：同摘要 PDF/DOCX 及验证通过状态。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- 真实渲染器验收检查 PDF 文本与页面、DOCX 内容与编辑。
- Workbench 授权测试和 Web 下载 E2E。

## Failure and unknown results

单一格式失败保留 staged 状态和错误，不只发布成功的一半为可投递。

## Parallel scheduling

- 理论前沿：G5；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
