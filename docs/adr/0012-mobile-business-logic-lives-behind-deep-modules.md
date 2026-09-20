---
status: accepted
---

# 移动业务逻辑放在深 Module 后，App 只负责组装与呈现

新原生移动端把 Deployment/Tenant scope、Task 提交与恢复、资源选择、Artifact、设备缓存和语音等行为收进少量深 Module，其 Interface 位于独立的 mobile-core 包；apps/mobile 只组装 Module、提供原生 Adapter 并呈现状态。我们不按页面建立各自的数据栈，也不让 Screen 直接组合 contracts、api-client 和存储，因为这种短期便利会把 scope、幂等、恢复与权限复杂度复制到每个调用方。
