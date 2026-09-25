package handler

// Pass B 宿主兼容层（b2-k-retrieval / K2.6）：retrieval handler 3 文件已物理迁移至
// internal/modules/knowledge/retrieval/app/handler（docs/plans/passb/22-knowledge-retrieval.md §5.5 行 3）。
// 本文件为留守宿主消费方（router/routes_knowledge.go:254/:279、routes_infra.go:14、
// container.go:197/:717/:721、router 测试 router_api_key_capabilities_test.go:301、
// semantic_internal_test.go:18）提供 type/var 别名，调用点零改动。
// 删除点：ib2 集成屏障直连后（Integration Brief 指令），随 manifest compat 登记行一并删除。

import (
	kbretrieval "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app/handler"
)

// --- type 别名（routes_knowledge.go:254/:279、routes_infra.go:14、container.go:197）---

type SemanticInternalHandler = kbretrieval.SemanticInternalHandler

type SemanticModelPolicyHandler = kbretrieval.SemanticModelPolicyHandler

type TagHandler = kbretrieval.TagHandler

// --- var 别名（container.go:198/:717/:721、router 测试构造面）---

var NewSemanticInternalHandler = kbretrieval.NewSemanticInternalHandler

var NewSemanticModelPolicyHandler = kbretrieval.NewSemanticModelPolicyHandler

var NewTagHandler = kbretrieval.NewTagHandler
