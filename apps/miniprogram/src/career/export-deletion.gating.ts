// T32 修复轮 M1：导出/删除主按钮门控，逐条对齐 Web ExportDeletionPage 的
// exportBlocked / deletionBlocked / startDeletionDisabled 语义——结果未知（恢复 intent
// 存续）与发起/对账/重试进行中都封锁主操作，防重复发起与竞态；导出未决还联动封锁
// 删除与知悉勾选（删除会吊销导出授权，导出结果必须先落定）。页面与测试共用本函数
// （可观察 seam：测试经真实 service 生命周期取输入，DevTools 复验渲染态）。
export interface LifecycleGatingInput {
  /** 发起导出或其恢复（对账/重试）正在进行（Web exportPhase === 'busy'）。 */
  exportBusy: boolean;
  /** 有结果未知的导出 intent（Web exportPhase === 'unknown'，恢复态置顶）。 */
  exportUnknown: boolean;
  /** 删除发起或其恢复（对账/重试）正在进行（Web deletionPhase === 'busy'）。 */
  deletionBusy: boolean;
  /** 有未落定的删除 intent 且页面未持有已呈报的 partial 回执（Web deletionPhase === 'unknown'；partial=确定回执不封锁）。 */
  deletionUnknown: boolean;
  /** 当前档案修订已读取（Web revision !== undefined）。 */
  revisionLoaded: boolean;
  /** 边界清单已呈现（Web !!boundary）。 */
  boundaryShown: boolean;
  /** 已勾选知悉。 */
  acknowledged: boolean;
  /** 已收到 deleted 终态回执（不可再次发起）。 */
  deleted: boolean;
}
export interface LifecycleGating {
  /** 导出主按钮与发起动作封锁（Web exportBlocked）。 */
  exportBlocked: boolean;
  /** 删除主按钮、发起与知悉勾选封锁（Web deletionBlocked，含导出未决联动）。 */
  deletionBlocked: boolean;
  exportDisabled: boolean;
  deletionDisabled: boolean;
  acknowledgeDisabled: boolean;
}
export function lifecycleGating(input: LifecycleGatingInput): LifecycleGating {
  const exportBlocked = input.exportBusy || input.exportUnknown;
  const deletionBlocked = input.deletionBusy || input.deletionUnknown || exportBlocked;
  return {
    exportBlocked,
    deletionBlocked,
    exportDisabled: exportBlocked || !input.revisionLoaded,
    deletionDisabled: deletionBlocked || !input.revisionLoaded || !input.boundaryShown || !input.acknowledged || input.deleted,
    acknowledgeDisabled: deletionBlocked,
  };
}

// 修复轮 2 F1：partial 确定回执后的恢复尝试（对账/重试）以未决告终时，页面持有的旧
// partial 回执不再代表当前结果——回到 unknown 封锁（Web ExportDeletionPage.tsx:267-268
// runDeletion uncertain → phase='unknown'；:286 lookupDeletionReceipt 失败 → phase='unknown'）。
/** 删除 unknown 门控输入：intent 存续且非已呈报 partial，或上次恢复尝试未决。 */
export function deletionOutcomeUnknown(input: { intentPresent: boolean; inMemoryStatus?: string; recoveryUnresolved: boolean }): boolean {
  return (input.intentPresent && input.inMemoryStatus !== 'partial') || input.recoveryUnresolved;
}
/** 恢复尝试失败后是否置未决：对账失败（intent 仍在）一律未决（Web lookup catch → unknown，
 * 回执读不到=结果未落定）；重试仅 ambiguous（outcome_unknown）未决——definite 拒绝走 Web
 * runDeletion 的 error 分支不封锁。intent 已不在当前作用域（SCOPE_CHANGED/终态清理）不置
 * 未决，避免无恢复入口时制造封锁死局。 */
export function deletionRecoveryUnresolvedAfter(kind: 'reconcile' | 'retry', error: unknown, intentStillPresent: boolean): boolean {
  if (!intentStillPresent) return false;
  if (kind === 'reconcile') return true;
  return (error as { code?: unknown } | null | undefined)?.code === 'outcome_unknown';
}

/** Stateful page transition seam. The page and behavior tests share the same receipt/unknown rules. */
export function createDeletionPageController() {
  let inMemoryStatus: string | undefined;
  let recoveryUnresolved = false;
  return {
    accept(status: string): void { inMemoryStatus = status; recoveryUnresolved = false; },
    clear(): void { inMemoryStatus = undefined; recoveryUnresolved = false; },
    initialOutcomeUnknown(): void { recoveryUnresolved = true; },
    recoveryFailed(kind: 'reconcile' | 'retry', error: unknown, intentStillPresent: boolean): void {
      recoveryUnresolved = deletionRecoveryUnresolvedAfter(kind, error, intentStillPresent);
    },
    isUnknown(intentPresent: boolean): boolean {
      return deletionOutcomeUnknown({ intentPresent, inMemoryStatus, recoveryUnresolved });
    },
    isRecoveryUnresolved(): boolean { return recoveryUnresolved; },
  };
}

/**
 * Confirm abandonment against the request shown when the modal opened. The
 * async modal can outlive that intent, so re-read identity and activity after
 * confirmation, then let the storage service perform a final synchronous CAS.
 */
export async function confirmAbandonIntent(requestId: string, options: {
  confirm: () => Promise<boolean>;
  currentRequestId: () => string | undefined;
  isBusy: () => boolean;
  abandon: (expectedRequestId: string) => boolean;
}): Promise<'abandoned' | 'cancelled' | 'changed' | 'busy'> {
  if (options.isBusy()) return 'busy';
  if (!await options.confirm()) return 'cancelled';
  if (options.currentRequestId() !== requestId) return 'changed';
  if (options.isBusy()) return 'busy';
  if (options.abandon(requestId)) return 'abandoned';
  return options.isBusy() ? 'busy' : 'changed';
}
