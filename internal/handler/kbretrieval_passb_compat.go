package handler

// Pass B 宿主兼容层（b2-k-retrieval / K2.6）：retrieval handler 3 文件曾物理迁移至
// internal/knowledge/retrieval/app/handler（docs/plans/passb/22-knowledge-retrieval.md §5.5 行 3）。
// order 60 handler 批：tag.go 与 tag_delete_test.go 已迁回本包（真身 TagHandler/
// NewTagHandler 同名同签名），Tag 侧别名随批删除；semantic_internal.go 与
// semantic_model_policy.go 仍留驻模块包（keepInPlace 待后续裁定），本文件继续为
// 留守宿主消费方（router/routes_infra.go:14、container.go 的 dig Provide、
// router_api_key_capabilities_test.go:301 等）提供 Semantic* 的 type/var 别名。
// 删除点：ib2 集成屏障直连后（Integration Brief 指令），随 manifest compat 登记行一并删除。

import (
	kbretrieval "github.com/Tencent/WeKnora/internal/knowledge/retrieval/app/handler"
)

// --- type 别名（routes_infra.go:14、container.go）---

type SemanticInternalHandler = kbretrieval.SemanticInternalHandler

type SemanticModelPolicyHandler = kbretrieval.SemanticModelPolicyHandler

// --- var 别名（container.go dig Provide、router 测试构造面）---

var NewSemanticInternalHandler = kbretrieval.NewSemanticInternalHandler

var NewSemanticModelPolicyHandler = kbretrieval.NewSemanticModelPolicyHandler
