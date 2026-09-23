# T29：Expo 移动端持续规则、额度与提醒

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#168](https://github.com/1123786563/WeKnora-fork01/issues/168)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）能管理持续找岗、查看额度与站内待办，并自主选择推送。

## Acceptance criteria

- [ ] 规则暂停后不再产生新搜索运行
- [ ] 收费执行前显示额度与预估，超额仍可读历史
- [ ] 系统推送不暴露公司、岗位或面试详情
- [ ] 空间切换和登出清理受控缓存

## Blocked by

T13、T20、T21、T23。

## Ownership and interfaces

- 主责边界：Expo 移动 Career 规则、用量与通知 Adapter。
- 消费接口：持续规则、额度、站内待办与 Mobile Runtime。
- 交付接口：原生规则、用量、推送选择与缓存失效。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 规则与额度测试。
- iOS、Android、鸿蒙各验证暂停、超额、通知权限和锁屏文案。

## Failure and unknown results

推送权限拒绝不妨碍站内待办。

## Parallel scheduling

- 理论前沿：G6；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
