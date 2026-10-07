package repository

// Pass B 宿主兼容层（b2-k-retrieval / K2.2）：retrieval 域 repository 5 文件已物理迁移至
// internal/modules/knowledge/retrieval/app/repository（docs/plans/passb/22-knowledge-retrieval.md §5.5 行 1）。
// 本文件为留守宿主消费方（container.go:192-195/:206/:1064、service/semantic_model_policy.go、
// service/semantic_scope.go 及宿主测试）提供 type/var 别名，调用点零改动。
// 删除点：ib2 集成屏障直连后（Integration Brief 指令），随 manifest 3 行 compat 登记一并删除。

import (
	kbretrieval "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app/repository"
)

// --- type 别名（container.go:195/:1064、service/semantic_scope.go:105/:114、
// service/semantic_model_policy.go:41-131 引用面）---

type SemanticModelPolicy = kbretrieval.SemanticModelPolicy

type SemanticModelPolicyRepository = kbretrieval.SemanticModelPolicyRepository

type SemanticControlRepository = kbretrieval.SemanticControlRepository

type SemanticModelInvocationStore = kbretrieval.SemanticModelInvocationStore

// --- var 别名（container.go:192-194/:206、service 测试引用面）---

var NewSemanticModelPolicyRepository = kbretrieval.NewSemanticModelPolicyRepository

var NewSemanticControlRepository = kbretrieval.NewSemanticControlRepository

var NewSemanticModelInvocationStore = kbretrieval.NewSemanticModelInvocationStore

// ErrSemanticModelPolicyNotFound 哨兵别名（service/semantic_model_policy_test.go 引用面）。
var ErrSemanticModelPolicyNotFound = kbretrieval.ErrSemanticModelPolicyNotFound
