/**
 * Task Office 共享合同（独立模块，R1-F21）：task-detail.ts 不得再从 task-office.ts
 * 值导入 TaskOfficeError——那会形成 task-office ⇄ task-detail 的运行时循环导入，
 * 在 ESM 初始化顺序下使 instanceof 判定失效。本模块无依赖，双方共同指向它。
 * AttentionState 同属两侧共享的展示合同，随迁至此（task-office.ts re-export 保持兼容）。
 */

export type AttentionState = 'none' | 'required';

export type TaskOfficeErrorCode =
  | 'TASK_OFFICE_SCOPE_CHANGED'
  | 'TASK_OFFICE_SUPERSEDED'
  | 'TASK_OFFICE_NO_ACTIVE_QUERY'
  | 'TASK_OFFICE_INVALID_INPUT'
  | 'TASK_OFFICE_BACKEND'
  | 'TASK_OFFICE_DETAIL_UNAVAILABLE'
  | 'TASK_OFFICE_DETAIL_CLOSED'
  | 'TASK_OFFICE_ATTACHMENTS_NOT_READY'
  | 'TASK_OFFICE_SUBMISSION_CONFLICT'
  | 'TASK_OFFICE_INTERACTIONS_UNAVAILABLE'
  | 'TASK_OFFICE_LEGACY_UNAVAILABLE'
  | 'TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE';

export class TaskOfficeError extends Error {
  constructor(readonly code: TaskOfficeErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'TaskOfficeError';
  }
}
