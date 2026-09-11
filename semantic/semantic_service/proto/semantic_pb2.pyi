from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RequestHeader(_message.Message):
    __slots__ = ("request_id", "trace_id")
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    TRACE_ID_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    trace_id: str
    def __init__(self, request_id: _Optional[str] = ..., trace_id: _Optional[str] = ...) -> None: ...

class ScopeKey(_message.Message):
    __slots__ = ("tenant_id", "kb_id")
    TENANT_ID_FIELD_NUMBER: _ClassVar[int]
    KB_ID_FIELD_NUMBER: _ClassVar[int]
    tenant_id: int
    kb_id: str
    def __init__(self, tenant_id: _Optional[int] = ..., kb_id: _Optional[str] = ...) -> None: ...

class DocumentRevision(_message.Message):
    __slots__ = ("scope", "document_id", "revision", "content_hash", "deleted")
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    DELETED_FIELD_NUMBER: _ClassVar[int]
    scope: ScopeKey
    document_id: str
    revision: int
    content_hash: str
    deleted: bool
    def __init__(self, scope: _Optional[_Union[ScopeKey, _Mapping]] = ..., document_id: _Optional[str] = ..., revision: _Optional[int] = ..., content_hash: _Optional[str] = ..., deleted: bool = ...) -> None: ...

class ChunkSnapshot(_message.Message):
    __slots__ = ("chunk_id", "text", "content_hash")
    CHUNK_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    chunk_id: str
    text: str
    content_hash: str
    def __init__(self, chunk_id: _Optional[str] = ..., text: _Optional[str] = ..., content_hash: _Optional[str] = ...) -> None: ...

class IndexConfig(_message.Message):
    __slots__ = ("config_digest", "engine_version", "model_profile_ref", "prompt_version", "rule_set_version", "schema_version")
    CONFIG_DIGEST_FIELD_NUMBER: _ClassVar[int]
    ENGINE_VERSION_FIELD_NUMBER: _ClassVar[int]
    MODEL_PROFILE_REF_FIELD_NUMBER: _ClassVar[int]
    PROMPT_VERSION_FIELD_NUMBER: _ClassVar[int]
    RULE_SET_VERSION_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    config_digest: str
    engine_version: str
    model_profile_ref: str
    prompt_version: str
    rule_set_version: str
    schema_version: str
    def __init__(self, config_digest: _Optional[str] = ..., engine_version: _Optional[str] = ..., model_profile_ref: _Optional[str] = ..., prompt_version: _Optional[str] = ..., rule_set_version: _Optional[str] = ..., schema_version: _Optional[str] = ...) -> None: ...

class ApplyRequest(_message.Message):
    __slots__ = ("document", "chunks", "manifest_ref", "config", "idempotency_key", "payload_hash")
    DOCUMENT_FIELD_NUMBER: _ClassVar[int]
    CHUNKS_FIELD_NUMBER: _ClassVar[int]
    MANIFEST_REF_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    IDEMPOTENCY_KEY_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_HASH_FIELD_NUMBER: _ClassVar[int]
    document: DocumentRevision
    chunks: _containers.RepeatedCompositeFieldContainer[ChunkSnapshot]
    manifest_ref: str
    config: IndexConfig
    idempotency_key: str
    payload_hash: str
    def __init__(self, document: _Optional[_Union[DocumentRevision, _Mapping]] = ..., chunks: _Optional[_Iterable[_Union[ChunkSnapshot, _Mapping]]] = ..., manifest_ref: _Optional[str] = ..., config: _Optional[_Union[IndexConfig, _Mapping]] = ..., idempotency_key: _Optional[str] = ..., payload_hash: _Optional[str] = ...) -> None: ...

class Operation(_message.Message):
    __slots__ = ("operation_id", "scope", "document_id", "revision", "state", "stage", "lease_token", "result_generation", "error_code")
    class State(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        OPERATION_STATE_UNSPECIFIED: _ClassVar[Operation.State]
        OPERATION_STATE_ACCEPTED: _ClassVar[Operation.State]
        OPERATION_STATE_RUNNING: _ClassVar[Operation.State]
        OPERATION_STATE_STAGED: _ClassVar[Operation.State]
        OPERATION_STATE_PUBLISHING: _ClassVar[Operation.State]
        OPERATION_STATE_SUCCEEDED: _ClassVar[Operation.State]
        OPERATION_STATE_FAILED: _ClassVar[Operation.State]
        OPERATION_STATE_CANCELLED: _ClassVar[Operation.State]
        OPERATION_STATE_SUPERSEDED: _ClassVar[Operation.State]
    OPERATION_STATE_UNSPECIFIED: Operation.State
    OPERATION_STATE_ACCEPTED: Operation.State
    OPERATION_STATE_RUNNING: Operation.State
    OPERATION_STATE_STAGED: Operation.State
    OPERATION_STATE_PUBLISHING: Operation.State
    OPERATION_STATE_SUCCEEDED: Operation.State
    OPERATION_STATE_FAILED: Operation.State
    OPERATION_STATE_CANCELLED: Operation.State
    OPERATION_STATE_SUPERSEDED: Operation.State
    OPERATION_ID_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    STAGE_FIELD_NUMBER: _ClassVar[int]
    LEASE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    RESULT_GENERATION_FIELD_NUMBER: _ClassVar[int]
    ERROR_CODE_FIELD_NUMBER: _ClassVar[int]
    operation_id: str
    scope: ScopeKey
    document_id: str
    revision: int
    state: Operation.State
    stage: str
    lease_token: int
    result_generation: str
    error_code: str
    def __init__(self, operation_id: _Optional[str] = ..., scope: _Optional[_Union[ScopeKey, _Mapping]] = ..., document_id: _Optional[str] = ..., revision: _Optional[int] = ..., state: _Optional[_Union[Operation.State, str]] = ..., stage: _Optional[str] = ..., lease_token: _Optional[int] = ..., result_generation: _Optional[str] = ..., error_code: _Optional[str] = ...) -> None: ...

class Evidence(_message.Message):
    __slots__ = ("evidence_id", "document_id", "revision", "chunk_id", "content_hash", "quote", "start_char", "end_char")
    EVIDENCE_ID_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    CHUNK_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    QUOTE_FIELD_NUMBER: _ClassVar[int]
    START_CHAR_FIELD_NUMBER: _ClassVar[int]
    END_CHAR_FIELD_NUMBER: _ClassVar[int]
    evidence_id: str
    document_id: str
    revision: int
    chunk_id: str
    content_hash: str
    quote: str
    start_char: int
    end_char: int
    def __init__(self, evidence_id: _Optional[str] = ..., document_id: _Optional[str] = ..., revision: _Optional[int] = ..., chunk_id: _Optional[str] = ..., content_hash: _Optional[str] = ..., quote: _Optional[str] = ..., start_char: _Optional[int] = ..., end_char: _Optional[int] = ...) -> None: ...

class Assertion(_message.Message):
    __slots__ = ("assertion_id", "scope", "subject_id", "predicate", "object_id", "value", "kind", "evidence_ids", "premise_ids", "valid_from", "valid_until")
    class Kind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        KIND_UNSPECIFIED: _ClassVar[Assertion.Kind]
        KIND_SOURCE: _ClassVar[Assertion.Kind]
        KIND_RULE: _ClassVar[Assertion.Kind]
        KIND_MODEL: _ClassVar[Assertion.Kind]
    KIND_UNSPECIFIED: Assertion.Kind
    KIND_SOURCE: Assertion.Kind
    KIND_RULE: Assertion.Kind
    KIND_MODEL: Assertion.Kind
    ASSERTION_ID_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_ID_FIELD_NUMBER: _ClassVar[int]
    PREDICATE_FIELD_NUMBER: _ClassVar[int]
    OBJECT_ID_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    EVIDENCE_IDS_FIELD_NUMBER: _ClassVar[int]
    PREMISE_IDS_FIELD_NUMBER: _ClassVar[int]
    VALID_FROM_FIELD_NUMBER: _ClassVar[int]
    VALID_UNTIL_FIELD_NUMBER: _ClassVar[int]
    assertion_id: str
    scope: ScopeKey
    subject_id: str
    predicate: str
    object_id: str
    value: str
    kind: Assertion.Kind
    evidence_ids: _containers.RepeatedScalarFieldContainer[str]
    premise_ids: _containers.RepeatedScalarFieldContainer[str]
    valid_from: str
    valid_until: str
    def __init__(self, assertion_id: _Optional[str] = ..., scope: _Optional[_Union[ScopeKey, _Mapping]] = ..., subject_id: _Optional[str] = ..., predicate: _Optional[str] = ..., object_id: _Optional[str] = ..., value: _Optional[str] = ..., kind: _Optional[_Union[Assertion.Kind, str]] = ..., evidence_ids: _Optional[_Iterable[str]] = ..., premise_ids: _Optional[_Iterable[str]] = ..., valid_from: _Optional[str] = ..., valid_until: _Optional[str] = ...) -> None: ...

class AccessScope(_message.Message):
    __slots__ = ("scope", "subject_id", "scope_ref", "scope_hash", "permission_epoch", "expires_at", "audience", "purpose", "budget_ref")
    class Purpose(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        PURPOSE_UNSPECIFIED: _ClassVar[AccessScope.Purpose]
        PURPOSE_SEARCH: _ClassVar[AccessScope.Purpose]
        PURPOSE_REASON: _ClassVar[AccessScope.Purpose]
        PURPOSE_INDEX: _ClassVar[AccessScope.Purpose]
    PURPOSE_UNSPECIFIED: AccessScope.Purpose
    PURPOSE_SEARCH: AccessScope.Purpose
    PURPOSE_REASON: AccessScope.Purpose
    PURPOSE_INDEX: AccessScope.Purpose
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_ID_FIELD_NUMBER: _ClassVar[int]
    SCOPE_REF_FIELD_NUMBER: _ClassVar[int]
    SCOPE_HASH_FIELD_NUMBER: _ClassVar[int]
    PERMISSION_EPOCH_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    AUDIENCE_FIELD_NUMBER: _ClassVar[int]
    PURPOSE_FIELD_NUMBER: _ClassVar[int]
    BUDGET_REF_FIELD_NUMBER: _ClassVar[int]
    scope: ScopeKey
    subject_id: str
    scope_ref: str
    scope_hash: str
    permission_epoch: int
    expires_at: str
    audience: str
    purpose: AccessScope.Purpose
    budget_ref: str
    def __init__(self, scope: _Optional[_Union[ScopeKey, _Mapping]] = ..., subject_id: _Optional[str] = ..., scope_ref: _Optional[str] = ..., scope_hash: _Optional[str] = ..., permission_epoch: _Optional[int] = ..., expires_at: _Optional[str] = ..., audience: _Optional[str] = ..., purpose: _Optional[_Union[AccessScope.Purpose, str]] = ..., budget_ref: _Optional[str] = ...) -> None: ...

class QueryLimits(_message.Message):
    __slots__ = ("max_hops", "max_nodes", "max_edges", "top_k", "max_tokens", "deadline_ms")
    MAX_HOPS_FIELD_NUMBER: _ClassVar[int]
    MAX_NODES_FIELD_NUMBER: _ClassVar[int]
    MAX_EDGES_FIELD_NUMBER: _ClassVar[int]
    TOP_K_FIELD_NUMBER: _ClassVar[int]
    MAX_TOKENS_FIELD_NUMBER: _ClassVar[int]
    DEADLINE_MS_FIELD_NUMBER: _ClassVar[int]
    max_hops: int
    max_nodes: int
    max_edges: int
    top_k: int
    max_tokens: int
    deadline_ms: int
    def __init__(self, max_hops: _Optional[int] = ..., max_nodes: _Optional[int] = ..., max_edges: _Optional[int] = ..., top_k: _Optional[int] = ..., max_tokens: _Optional[int] = ..., deadline_ms: _Optional[int] = ...) -> None: ...

class SearchRequest(_message.Message):
    __slots__ = ("header", "query_id", "query", "access_scope", "limits", "mode")
    HEADER_FIELD_NUMBER: _ClassVar[int]
    QUERY_ID_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    ACCESS_SCOPE_FIELD_NUMBER: _ClassVar[int]
    LIMITS_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    header: RequestHeader
    query_id: str
    query: str
    access_scope: AccessScope
    limits: QueryLimits
    mode: str
    def __init__(self, header: _Optional[_Union[RequestHeader, _Mapping]] = ..., query_id: _Optional[str] = ..., query: _Optional[str] = ..., access_scope: _Optional[_Union[AccessScope, _Mapping]] = ..., limits: _Optional[_Union[QueryLimits, _Mapping]] = ..., mode: _Optional[str] = ...) -> None: ...

class SearchResponse(_message.Message):
    __slots__ = ("query_id", "generation", "mode", "evidence", "assertion_ids", "paths", "stale", "partial", "truncated")
    class Path(_message.Message):
        __slots__ = ("nodes",)
        NODES_FIELD_NUMBER: _ClassVar[int]
        nodes: _containers.RepeatedScalarFieldContainer[str]
        def __init__(self, nodes: _Optional[_Iterable[str]] = ...) -> None: ...
    QUERY_ID_FIELD_NUMBER: _ClassVar[int]
    GENERATION_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    EVIDENCE_FIELD_NUMBER: _ClassVar[int]
    ASSERTION_IDS_FIELD_NUMBER: _ClassVar[int]
    PATHS_FIELD_NUMBER: _ClassVar[int]
    STALE_FIELD_NUMBER: _ClassVar[int]
    PARTIAL_FIELD_NUMBER: _ClassVar[int]
    TRUNCATED_FIELD_NUMBER: _ClassVar[int]
    query_id: str
    generation: str
    mode: str
    evidence: _containers.RepeatedCompositeFieldContainer[Evidence]
    assertion_ids: _containers.RepeatedScalarFieldContainer[str]
    paths: _containers.RepeatedCompositeFieldContainer[SearchResponse.Path]
    stale: bool
    partial: bool
    truncated: bool
    def __init__(self, query_id: _Optional[str] = ..., generation: _Optional[str] = ..., mode: _Optional[str] = ..., evidence: _Optional[_Iterable[_Union[Evidence, _Mapping]]] = ..., assertion_ids: _Optional[_Iterable[str]] = ..., paths: _Optional[_Iterable[_Union[SearchResponse.Path, _Mapping]]] = ..., stale: bool = ..., partial: bool = ..., truncated: bool = ...) -> None: ...

class ReasonRequest(_message.Message):
    __slots__ = ("header", "search", "reasoning_mode", "rule_set_version")
    class ReasoningMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        REASONING_MODE_UNSPECIFIED: _ClassVar[ReasonRequest.ReasoningMode]
        REASONING_MODE_RULES: _ClassVar[ReasonRequest.ReasoningMode]
        REASONING_MODE_MODEL: _ClassVar[ReasonRequest.ReasoningMode]
    REASONING_MODE_UNSPECIFIED: ReasonRequest.ReasoningMode
    REASONING_MODE_RULES: ReasonRequest.ReasoningMode
    REASONING_MODE_MODEL: ReasonRequest.ReasoningMode
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    REASONING_MODE_FIELD_NUMBER: _ClassVar[int]
    RULE_SET_VERSION_FIELD_NUMBER: _ClassVar[int]
    header: RequestHeader
    search: SearchRequest
    reasoning_mode: ReasonRequest.ReasoningMode
    rule_set_version: str
    def __init__(self, header: _Optional[_Union[RequestHeader, _Mapping]] = ..., search: _Optional[_Union[SearchRequest, _Mapping]] = ..., reasoning_mode: _Optional[_Union[ReasonRequest.ReasoningMode, str]] = ..., rule_set_version: _Optional[str] = ...) -> None: ...

class ReasonResponse(_message.Message):
    __slots__ = ("retrieval", "status", "conclusion", "conclusion_kind", "premise_ids", "rule_ids", "model_version", "prompt_version", "limitations")
    class ConclusionKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        CONCLUSION_KIND_UNSPECIFIED: _ClassVar[ReasonResponse.ConclusionKind]
        CONCLUSION_KIND_RULE: _ClassVar[ReasonResponse.ConclusionKind]
        CONCLUSION_KIND_MODEL: _ClassVar[ReasonResponse.ConclusionKind]
    CONCLUSION_KIND_UNSPECIFIED: ReasonResponse.ConclusionKind
    CONCLUSION_KIND_RULE: ReasonResponse.ConclusionKind
    CONCLUSION_KIND_MODEL: ReasonResponse.ConclusionKind
    RETRIEVAL_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    CONCLUSION_FIELD_NUMBER: _ClassVar[int]
    CONCLUSION_KIND_FIELD_NUMBER: _ClassVar[int]
    PREMISE_IDS_FIELD_NUMBER: _ClassVar[int]
    RULE_IDS_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    PROMPT_VERSION_FIELD_NUMBER: _ClassVar[int]
    LIMITATIONS_FIELD_NUMBER: _ClassVar[int]
    retrieval: SearchResponse
    status: str
    conclusion: str
    conclusion_kind: ReasonResponse.ConclusionKind
    premise_ids: _containers.RepeatedScalarFieldContainer[str]
    rule_ids: _containers.RepeatedScalarFieldContainer[str]
    model_version: str
    prompt_version: str
    limitations: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, retrieval: _Optional[_Union[SearchResponse, _Mapping]] = ..., status: _Optional[str] = ..., conclusion: _Optional[str] = ..., conclusion_kind: _Optional[_Union[ReasonResponse.ConclusionKind, str]] = ..., premise_ids: _Optional[_Iterable[str]] = ..., rule_ids: _Optional[_Iterable[str]] = ..., model_version: _Optional[str] = ..., prompt_version: _Optional[str] = ..., limitations: _Optional[_Iterable[str]] = ...) -> None: ...

class Capability(_message.Message):
    __slots__ = ("mode", "available", "unavailable_reason")
    MODE_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    UNAVAILABLE_REASON_FIELD_NUMBER: _ClassVar[int]
    mode: str
    available: bool
    unavailable_reason: str
    def __init__(self, mode: _Optional[str] = ..., available: bool = ..., unavailable_reason: _Optional[str] = ...) -> None: ...

class GetCapabilitiesRequest(_message.Message):
    __slots__ = ("header",)
    HEADER_FIELD_NUMBER: _ClassVar[int]
    header: RequestHeader
    def __init__(self, header: _Optional[_Union[RequestHeader, _Mapping]] = ...) -> None: ...

class GetCapabilitiesResponse(_message.Message):
    __slots__ = ("protocol_version", "engine_version", "capabilities", "default_limits")
    PROTOCOL_VERSION_FIELD_NUMBER: _ClassVar[int]
    ENGINE_VERSION_FIELD_NUMBER: _ClassVar[int]
    CAPABILITIES_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_LIMITS_FIELD_NUMBER: _ClassVar[int]
    protocol_version: str
    engine_version: str
    capabilities: _containers.RepeatedCompositeFieldContainer[Capability]
    default_limits: QueryLimits
    def __init__(self, protocol_version: _Optional[str] = ..., engine_version: _Optional[str] = ..., capabilities: _Optional[_Iterable[_Union[Capability, _Mapping]]] = ..., default_limits: _Optional[_Union[QueryLimits, _Mapping]] = ...) -> None: ...

class ApplyDocumentRevisionRequest(_message.Message):
    __slots__ = ("header", "apply")
    HEADER_FIELD_NUMBER: _ClassVar[int]
    APPLY_FIELD_NUMBER: _ClassVar[int]
    header: RequestHeader
    apply: ApplyRequest
    def __init__(self, header: _Optional[_Union[RequestHeader, _Mapping]] = ..., apply: _Optional[_Union[ApplyRequest, _Mapping]] = ...) -> None: ...

class DeleteDocumentRequest(_message.Message):
    __slots__ = ("header", "document")
    HEADER_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_FIELD_NUMBER: _ClassVar[int]
    header: RequestHeader
    document: DocumentRevision
    def __init__(self, header: _Optional[_Union[RequestHeader, _Mapping]] = ..., document: _Optional[_Union[DocumentRevision, _Mapping]] = ...) -> None: ...

class GetOperationRequest(_message.Message):
    __slots__ = ("header", "scope", "operation_id")
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    OPERATION_ID_FIELD_NUMBER: _ClassVar[int]
    header: RequestHeader
    scope: ScopeKey
    operation_id: str
    def __init__(self, header: _Optional[_Union[RequestHeader, _Mapping]] = ..., scope: _Optional[_Union[ScopeKey, _Mapping]] = ..., operation_id: _Optional[str] = ...) -> None: ...

class CancelOperationRequest(_message.Message):
    __slots__ = ("header", "scope", "operation_id")
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    OPERATION_ID_FIELD_NUMBER: _ClassVar[int]
    header: RequestHeader
    scope: ScopeKey
    operation_id: str
    def __init__(self, header: _Optional[_Union[RequestHeader, _Mapping]] = ..., scope: _Optional[_Union[ScopeKey, _Mapping]] = ..., operation_id: _Optional[str] = ...) -> None: ...
