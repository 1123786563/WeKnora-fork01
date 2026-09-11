# Semantica 协议与服务 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立不依赖上游内部对象的服务契约及可认证通信。

**Architecture:** 生产包只使用V01锁定依赖，Go与Python显式映射；证据模型先于持久化。

**Tech Stack:** Go/Gin/GORM、Python/gRPC/Semantica、PostgreSQL/Neo4j、React/TypeScript；按涉及范围使用。

**Spec:** [架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)；[总计划与完整类型表](2026-09-11-semantica-implementation.md)。

## Global Constraints

完整继承总计划 Global Constraints，必须先读；本计划不扩大语义服务所有权、授权范围或首版能力。所有代码和测试均为后续实施输入，未执行。

---


## C01：版本化协议和跨语言领域类型

**依赖：** V01。

**文件与职责：**

- `semantic/pyproject.toml`：C01建立包布局、已验证依赖与pytest开发依赖
- `semantic/uv.lock`：C01生成独立生产包锁，C02扩展时受控更新
- `semantic/semantic_service/__init__.py`：生产Python包入口
- `semantic/proto/semantic.proto`：唯一 wire schema
- `semantic/proto/semantic.pb.go`：Go 消息生成物
- `semantic/proto/semantic_grpc.pb.go`：Go RPC生成物
- `semantic/semantic_service/proto/`：Python 生成物
- `semantic/semantic_service/contracts.py`：Python 领域 DTO
- `internal/types/semantic.go`：Go 领域 DTO
- `internal/types/interfaces/semantic.go`：Go service interfaces
- `semantic/scripts/generate_proto.sh`：固定生成器的单一生成入口
- `semantic/tests/test_contract.py`：Python golden校验
- `internal/infrastructure/semantic/contract_test.go`：Go golden校验
- `semantic/tests/fixtures/contract-v1.json`：共享跨语言样本

**接口：** 实现总计划4.1全部 DTO 与7个RPC；Go SemanticClient 接口 Apply(ctx,types.SemanticApplyRequest)(types.SemanticOperation,error)、Delete(ctx,types.SemanticDocumentRevision)(types.SemanticOperation,error)、Get/Cancel(ctx,types.SemanticScopeKey,string)(types.SemanticOperation,error)、Search(ctx,types.SemanticSearchRequest)(types.SemanticSearchResponse,error)、Reason(ctx,types.SemanticReasonRequest)(types.SemanticReasonResponse,error)、Capabilities(ctx)(types.SemanticCapabilities,error)。ctx均为context.Context。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_optional_evidence_span_is_not_zero():
    evidence = Evidence(evidence_id="e1", document_id="d1", revision=1,
        chunk_id="c1", content_hash="h", quote="甲", start_char=None, end_char=None)
    assert evidence.start_char is None
    assert evidence.revision == 1
# Go golden 测试必须额外 round-trip uint64 最大值，tenant/revision 不经 float64。
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_contract.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 逐字段定义枚举、optional、oneof与RPC envelope；tenant和revision使用uint64，JSON公共边界把64位整数编码为十进制字符串**

- [ ] **4. 用同一 proto 生成 Go/Python；固定生成器版本并写脚本，旧tag删除后reserved；Golden覆盖空span、中文、unknown enum、最大uint64和错误detail**

- [ ] **5. 新增Go接口及显式映射，不把protobuf对象穿透业务层；记录schema版本兼容规则与capability协商失败行为**

关键实现约束：

```
syntax = "proto3";
package weknora.semantic.v1;
option go_package = "github.com/Tencent/WeKnora/semantic/proto;semanticpb";
message Span {
  optional uint32 start_char = 1;
  optional uint32 end_char = 2;
}
message ScopeKey {
  uint64 tenant_id = 1;
  string kb_id = 2;
}
message DocumentRevision {
  ScopeKey scope = 1;
  string document_id = 2;
  uint64 revision = 3;
  string content_hash = 4;
  bool deleted = 5;
}
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_contract.py -q`，预期退出码 0；另完成：执行 go test ./internal/infrastructure/semantic -run TestContract -count=1；二次生成 git diff 无变化；不要求未知枚举映射成默认成功。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 C01 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): c01 版本化协议和跨语言领域类型`。

## C02：认证服务骨架和Go客户端

**依赖：** C01。

**文件与职责：**

- `semantic/pyproject.toml`：扩展C01包配置与已验证精确依赖
- `semantic/uv.lock`：扩展C01生产依赖锁
- `semantic/semantic_service/server.py`：gRPC入口与健康检查
- `semantic/semantic_service/auth.py`：内部认证拦截器
- `semantic/semantic_service/config.py`：服务配置
- `semantic/tests/conftest.py`：真实临时gRPC rpc_client fixture
- `semantic/tests/test_rpc_auth.py`：认证/取消测试
- `internal/infrastructure/semantic/client.go`：Go deadline/error/认证适配
- `internal/infrastructure/semantic/client_test.go`：Go transport测试
- `internal/config/config.go`：可选SEMANTIC配置
- `internal/container/container.go`：可关闭的依赖注入

**接口：** 新增 NewClient(config SemanticClientConfig)(interfaces.SemanticClient,error)、Close()error；Python create_server(config)->grpc.Server。rpc_client fixture启动随机本地端口和测试证书，缺认证不提供业务RPC；未实现方法返回UNIMPLEMENTED而不是假成功。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_missing_service_identity_is_denied(rpc_client):
    with pytest.raises(grpc.RpcError) as exc:
        rpc_client.unauthenticated.GetCapabilities(GetCapabilitiesRequest())
    assert exc.value.code() == grpc.StatusCode.UNAUTHENTICATED
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_rpc_auth.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 建立server启动/关闭、健康探针与认证拦截器；生产缺TLS或内部认证配置时启动失败；测试证书仅测试使用**

- [ ] **4. Go客户端传递trace、deadline、取消，映射标准错误；仅Get/Capabilities等读操作可透明重试，Apply由业务按幂等键重试**

- [ ] **5. 注册可选服务配置；enabled=false时原系统可启动，禁止因client连接对象存在就判定服务ready**

关键实现约束：

```
if not verified_service_identity(context):
    context.abort(grpc.StatusCode.UNAUTHENTICATED, "service identity required")
if context.time_remaining() is not None and context.time_remaining() <= 0:
    context.abort(grpc.StatusCode.DEADLINE_EXCEEDED, "deadline exceeded")
# verified_service_identity 在 auth.py 验证 mTLS 身份或批准的内部token及受众。
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_rpc_auth.py -q`，预期退出码 0；另完成：Go TestSemanticClient 覆盖连接失败、deadline、取消与UNIMPLEMENTED；健康readiness真实反映依赖，不公开宿主机业务端口。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 C02 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): c02 认证服务骨架和Go客户端`。

## C03：事实与证据校验模型

**依赖：** C01,V02。

**文件与职责：**

- `semantic/semantic_service/evidence.py`：中文quote/span校验
- `semantic/semantic_service/facts.py`：事实与实体身份模型
- `semantic/tests/test_evidence.py`：原文位置测试
- `semantic/tests/test_facts.py`：来源/冲突/租户身份测试

**接口：** 定义 validate_evidence(chunk:ChunkSnapshot,evidence:Evidence)->None；new_entity_id()->str 返回UUID；validate_assertion(assertion:Assertion,evidence_by_id:dict,premises_by_id:dict)->None。实体名称/别名以有来源assertion表达，object_id/value恰有一个。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_span_uses_unicode_codepoints():
    assert extract_quote("甲😀乙", 1, 2) == "😀"
    with pytest.raises(ValueError):
        validate_span("甲😀乙", 1, 2, "乙")

def test_invalid_span_is_not_silently_repaired():
    with pytest.raises(ValueError):
        validate_span("甲乙", 2, 1, "")
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_evidence.py semantic/tests/test_facts.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 实现extract_quote(text,start,end)->str和validate_span；边界为Unicode codepoint半开区间，两端同时存在或同时为空；无位置quote须是原文子串**

- [ ] **4. 将原始实体UUID、等价断言、来源事实、规则/模型推导分开；记录配置版本，校验跨scope引用与悬空premise/evidence**

- [ ] **5. 拒绝错误哈希、冲突object/value、循环推导DAG；同subject/predicate不同值保留冲突，不按最后写入覆盖**

关键实现约束：

```
def extract_quote(text, start, end):
    if not 0 <= start <= end <= len(text):
        raise ValueError("invalid codepoint span")
    return text[start:end]

def validate_span(text, start, end, quote):
    if extract_quote(text, start, end) != quote:
        raise ValueError("quote mismatch")
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_evidence.py semantic/tests/test_facts.py -q`，预期退出码 0；另完成：中文/emoji、空span、篡改quote、跨租户引用、证据缺失和冲突并存均有断言；为后续删除保留完整支持关系。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 C03 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): c03 事实与证据校验模型`。
