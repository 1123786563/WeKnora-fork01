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
