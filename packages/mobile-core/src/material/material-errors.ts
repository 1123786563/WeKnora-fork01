/**
 * Task Material 共享错误合同（独立于实现文件，避免循环导入；镜像
 * task-office-errors.ts 的模式）。Message 即裸错误码，呈现层查文案表。
 */
export type MaterialErrorCode =
  | 'MATERIAL_SCOPE_CHANGED'
  | 'MATERIAL_CLOSED'
  | 'MATERIAL_INVALID_INPUT'
  | 'MATERIAL_NOT_FOUND'
  | 'MATERIAL_GRANT_EXPIRED'
  | 'MATERIAL_GRANT_INVALID'
  | 'MATERIAL_SIGNING_DISABLED'
  | 'MATERIAL_TERMINAL_READ_ONLY'
  | 'MATERIAL_GRANT_ORIGIN'
  | 'MATERIAL_SHARE_UNAVAILABLE'
  | 'MATERIAL_BACKEND';

export class MaterialError extends Error {
  constructor(readonly code: MaterialErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'MaterialError';
  }
}
