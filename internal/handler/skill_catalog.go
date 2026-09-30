// Pass B 25b 过渡占位残差（IB2 删除）：本文件内容已随租户 Skill handler 面
// 迁往 internal/modules/agentcatalog/handler/skill_catalog.go。
// 保留占位以满足 manifest legacy_files 存在性核验（tools/modulemove）；
// 本节点在此路径不留任何业务声明——8 个端点方法经宿主 skill_handler.go 的
// SkillHandler 别名整体可达（Go 不可经别名重声明方法，见该文件注释）。
// remove_at: ib2。
package handler
