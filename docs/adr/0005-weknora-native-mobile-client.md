# 原生移动端以 WeKnora 领域模型重新建设

移动 AI Office 在当前仓库新建 `apps/mobile`，复用 WeKnora 的 contracts、API client、domain、设计令牌和国际化，不 fork Happy 或 Paseo 整体客户端。实现可以选择性迁移 Paseo 的移动布局、时间线、Diff、文件浏览、键盘和语音原生模块，并借鉴 Happy 的任务交互、审批、附件与恢复体验，但 Tenant、Task、权限、连接和云端执行始终使用 WeKnora 自身模型；迁移代码保留许可证与来源记录。
