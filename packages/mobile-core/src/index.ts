export { createMobileRuntime } from './runtime/mobile-runtime.ts';
export { createInMemoryCredentialStore, createInMemoryDeploymentRegistry } from './runtime/in-memory-adapters.ts';
export type { AppLifecyclePort, AuthorizedStreamTransport, AuthorizedTransport, CredentialStore, DeploymentRegistry, DeploymentStore, MobileRuntimePorts, OidcBrowserPort, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './runtime/ports.ts';
export type { Deployment, DeploymentInput, MobileRuntime, RuntimeReason, RuntimeSnapshot, RuntimeSurface, ScopeLease, TenantOption } from './runtime/types.ts';
export { createScopedVault, scopeKeyOf } from './vault/scoped-vault.ts';
export { base64ToBytes, bytesToBase64 } from './vault/base64.ts';
export { createWebCryptoCipher } from './vault/web-crypto-cipher.ts';
export { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from './vault/in-memory-adapters.ts';
export type { CipherPort, KeyStorePort, ScopedVaultPorts, VaultStoragePort } from './vault/ports.ts';
export type { DraftEntry, ScopedDraftRepository, ScopedStore, ScopedVault, VaultPolicy, VaultRevokeReason } from './vault/scoped-vault.ts';
export { createResourceShelf } from './shelf/resource-shelf.ts';
export { createInMemoryResourceRemote } from './shelf/in-memory-resource-remote.ts';
export type { ResourceRemote, ResourceShelfPorts } from './shelf/ports.ts';
export type { ResourceClass, ResourceClassVerdict, ResourcePage, ResourceQuery, ResourceShelf, ResourceShelfHandle, SelectionVerdict, ShelfCloseReason, ShelfInvalidationEvent } from './shelf/types.ts';
export { createTaskOffice, TaskOfficeError } from './task-office/task-office.ts';
export { createInMemoryIntentLog } from './task-office/task-office.ts';
export { createScenarioTaskBackend, emptyOverview } from './task-office/in-memory-task-backend.ts';
export type {
  AttentionState, HomeView, InteractionCard, SubmissionIntentLog, SubmissionIntentRecord, TaskBackendListInput, TaskBackendLookup, TaskBackendOverview, TaskBackendPage,
  TaskBackendPort, TaskBackendRun, TaskBackendStartAck, TaskBackendStartInput, TaskCard, TaskListPage, TaskOffice, TaskOfficeErrorCode,
  TaskOfficeGoal, TaskOfficePorts, TaskOfficeQuery, TaskStartReceipt, TaskStatusFilter,
} from './task-office/task-office.ts';
export { createAttentionDecider, interactionActionAllowed, INTERACTION_ACTIONS } from './task-office/attention-inbox.ts';
// 集成说明（T08 #38 × #41）：两分支各自定义了 InboxItem/InboxView。行动通知收件箱（#41，
// inbox/notification-inbox.ts）保留 InboxItem/InboxView 本名；审批交互收件箱（T08，
// task-office/attention-inbox.ts）在 barrel 以 AttentionInboxItem/AttentionInboxView 别名导出，
// 消除重复标识符，两者语义并存。
export type {
  AttentionDecisionInput, AttentionDecisionReceipt, AttentionDeciderDeps,
  InboxItem as AttentionInboxItem, InboxView as AttentionInboxView,
  InteractionActionValue, InteractionBackendPort, InteractionKindValue, ResolvedDecisionRecord,
} from './task-office/attention-inbox.ts';
export type { ScenarioTaskBackend, ScenarioTaskBackendHandlers } from './task-office/in-memory-task-backend.ts';
export { createScriptedTaskStream, createScenarioTaskDetailBackend, createInMemoryTaskProjectionStore } from './task-office/in-memory-task-detail.ts';
export { createTaskDetail, TASK_DETAIL_HISTORY_LIMIT } from './task-office/task-detail.ts';
export type {
  OfflineTaskSnapshot, PersistedTaskProjection, TaskBackendDetail, TaskBackendEvent, TaskCommandAction, TaskCommandPort, TaskConnectionState, TaskDetailView,
  TaskDetailBackendPort, TaskDetailPorts, TaskHandle, TaskInterruptionReason, TaskProjectionStore, TaskStreamControlFrame,
} from './task-office/task-detail.ts';
export { resolveUnknownStop } from './task-office/task-intent.ts';
export type { InterventionOutcome, InterventionReceipt, StopPhase, TaskIntent } from './task-office/task-intent.ts';
export { createVaultTaskProjectionStore, PROJECTION_BODY_BUDGET_BYTES } from './task-office/vault-projection-store.ts';
export type { TaskLifecycleState, TaskTimelineEntry, TaskTimelineKind, TaskTimelineSourceEvent } from './task-office/task-timeline.ts';
export { isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf, timelineKindLabel } from './task-office/task-timeline.ts';
export type { ScenarioTaskDetailHandlers, ScriptedTaskStream } from './task-office/in-memory-task-detail.ts';
export { parseNotificationDeepLink } from './inbox/deep-link.ts';
export type { DeepLinkTarget, TaskDetailDeepLinkTarget } from './inbox/deep-link.ts';
export { createDeviceRegistry, DeviceError } from './device/device-registry.ts';
export type { DeviceErrorCode, DevicePlatform, DevicePorts, DeviceRegistrationRecord, DeviceRegistry, DeviceRemote } from './device/device-registry.ts';
export { createScenarioDeviceRemote } from './device/in-memory-device-remote.ts';
export type { ScenarioDeviceRemote, ScenarioDeviceSnapshot } from './device/in-memory-device-remote.ts';
export { createNotificationInbox, InboxError } from './inbox/notification-inbox.ts';
export type {
  InboxBackendItem, InboxBackendPage, InboxErrorCode, InboxItem, InboxRemote, InboxView,
  NotificationInbox, NotificationInboxPorts,
} from './inbox/notification-inbox.ts';
export { createScenarioInboxRemote } from './inbox/in-memory-inbox-remote.ts';
export type { InboxScriptPage, ScenarioInboxRemote } from './inbox/in-memory-inbox-remote.ts';
export { legacyTaskGates, LEGACY_TASK_NEW_RUN_REASON, createScenarioLegacyTaskBackend } from './task-office/legacy-tasks.ts';
export type {
  LegacyBackendTask, LegacyFollowUpInput, LegacyMessage, LegacyTaskBackendPage, LegacyTaskBackendPort,
  LegacyTaskCapability, LegacyTaskCard, LegacyTaskGates, LegacyTaskIntent, LegacyTaskListPage,
  ScenarioLegacyTaskBackend, ScenarioLegacyTaskBackendHandlers,
} from './task-office/legacy-tasks.ts';
export { createTaskMaterial } from './material/task-material.ts';
export { MaterialError } from './material/material-errors.ts';
export type { MaterialErrorCode } from './material/material-errors.ts';
export { materialKindOf, previewVerdictOf, isInlineImageMime, PREVIEW_MAX_BYTES } from './material/material-kinds.ts';
export { parseUnifiedDiff } from './material/diff.ts';
export type { DiffHunk, DiffLine } from './material/diff.ts';
export { projectCitations } from './material/evidence.ts';
export { createScenarioMaterialRemote, createScriptedBlobFetch, createRecordingSharePort } from './material/in-memory-material-remote.ts';
export type { ScenarioMaterialHandlers } from './material/in-memory-material-remote.ts';
export type {
  BlobFetchPort, MaterialBackendArtifact, MaterialBackendEvent, MaterialBackendGrant, MaterialBackendList,
  MaterialBackendPort, MaterialBackendTerminalLine, MaterialBackendTerminalPage, SharePort, TaskMaterialPorts,
} from './material/ports.ts';
export type {
  EvidenceCitation, MaterialActResult, MaterialEntry, MaterialEntryKind, MaterialEvent, MaterialIndex,
  MaterialIntent, MaterialRef, MaterialView, PreviewVerdict, TaskMaterial, TaskMaterialHandle, TerminalLine,
} from './material/types.ts';
export { createTaskBudgetOps, TaskBudgetError } from './task-office/task-budget.ts';
export type {
  TaskBudgetBackendPort, TaskBudgetErrorCode, TaskBudgetExtendInput, TaskBudgetExtendReceipt, TaskBudgetFacts,
} from './task-office/task-budget.ts';
export { createScenarioTaskBudgetBackend } from './task-office/in-memory-task-budget.ts';
export type { ScenarioTaskBudgetScript } from './task-office/in-memory-task-budget.ts';
export { createOfflineGate, OfflineGateError, OFFLINE_ACTION_BLOCKED } from './offline/offline-gate.ts';
export type { NetworkStatusPort, OfflineActionKind, OfflineGate } from './offline/offline-gate.ts';
export { guardInteractionBackend, guardKnowledgeQABackend, guardLegacyTaskBackend, guardTaskBackend } from './offline/guarded-ports.ts';
// T15（#45）：知识问答证据域与 Task Office askKnowledge 端口。
// 集成说明（T15 #45 × material #T11）：material 域已占用本名导出 EvidenceCitation（material/types.ts）。
// 知识问答引用在 barrel 以 KnowledgeQAEvidenceCitation 别名导出（与上方 AttentionInboxItem 同先例），
// knowledge-qa.ts 模块内类型名保持 EvidenceCitation 不变。
export { EVIDENCE_KIND_LABEL, createScenarioKnowledgeQABackend, evidenceRetryable } from './task-office/knowledge-qa.ts';
export type {
  AnswerEvidenceStateValue, AnswerEvidenceView, EvidenceCitation as KnowledgeQAEvidenceCitation, EvidenceConclusion,
  EvidenceKindValue, EvidenceReasoning, EvidenceReasoningStateValue,
  KnowledgeQAAskInput, KnowledgeQABackendPort, KnowledgeQATurn, KnowledgeQATurnBody,
  ScenarioKnowledgeQABackend, ScenarioKnowledgeQABackendHandlers,
} from './task-office/knowledge-qa.ts';
// —— T22 (#52) 代码交付只读投影 ——
export { deliveryViewOf } from './delivery/delivery-view.ts';
export type { DeliveryReceiptView, DeliveryRemoteRecord, DeliveryState } from './delivery/delivery-view.ts';
export { createDeliveryReader, DeliveryReaderError } from './delivery/delivery-reader.ts';
export type { DeliveryReader, DeliveryReaderErrorCode, DeliveryRemote } from './delivery/delivery-reader.ts';
export { createDeliveryRecovery, DeliveryRecoveryError } from './delivery/delivery-recovery.ts';
export type { DeliveryRecovery, DeliveryRecoveryErrorCode, DeliveryRecoveryRemote } from './delivery/delivery-recovery.ts';
export { createScenarioDeliveryRemote } from './delivery/in-memory-delivery-remote.ts';

export { createDictation, DICTATION_MAX_AUDIO_BYTES, DICTATION_MAX_DURATION_MS } from './voice/dictation.ts';
export type {
  Dictation, DictationAudio, DictationCapturePort, DictationCaptureStart, DictationFailure,
  DictationPhase, DictationPorts, DictationState, DictationTranscriptionInput,
  DictationTranscriptionPort, DictationTranscriptionResult,
} from './voice/dictation.ts';
export { createScenarioDictationTranscriber, createScriptedDictationCapture } from './voice/in-memory-dictation.ts';
export type { ScenarioDictationTranscriber, ScriptedDictationCapture, ScriptedDictationCaptureOptions } from './voice/in-memory-dictation.ts';
// —— T17 (#47) 只读研究委派与版本化批注 ——
export { ResearchError, createTaskResearch } from './research/task-research.ts';
export type { ResearchErrorCode } from './research/task-research.ts';
export type {
  ResearchAnnotationDraft, ResearchAnnotationReceipt, ResearchAnnotationRow, ResearchBackendPort,
  ResearchDelegationRow, ResearchDraftsPort, ResearchEvent, ResearchRevisionInput, ResearchRevisionReceipt,
  ResearchStatus, TaskResearch, TaskResearchHandle, TaskResearchPorts,
} from './research/types.ts';
export { createScenarioResearchRemote } from './research/in-memory-research-remote.ts';
export type { ScenarioResearchRemoteScript } from './research/in-memory-research-remote.ts';

// —— T27 (#57) Task 内实时语音会话（Voice Room，module-seams §8）——
export { createVoiceRoom, VoiceRoomError, VOICE_ROOM_MAX_AUDIO_BYTES, VOICE_ROOM_MAX_SECONDS } from './voice-room/voice-room.ts';
export type {
  VoiceAudioDisposition, VoiceHandle, VoiceRoom, VoiceRoomErrorCode, VoiceRoomNoticeReason,
  VoiceRoomPhase, VoiceRoomPorts, VoiceRoomState, VoiceSessionEndReceipt, VoiceSessionGrant,
  VoiceSessionPort, VoiceSteerIntent, VoiceTurnTranscriptionPort, VoiceTurnView,
} from './voice-room/voice-room.ts';
export { createScriptedVoiceSession } from './voice-room/in-memory-voice-session.ts';
export type { ScriptedVoiceSession } from './voice-room/in-memory-voice-session.ts';
