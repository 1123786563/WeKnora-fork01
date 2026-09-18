// Verbatim port of frontend/src/views/apps/actionState.ts: the approval
// control mapping (approve needs awaiting_approval AND permission; execute
// needs authorized AND permission; retry is ALWAYS false in phase one —
// unknown shows 结果待核对 and no resend is ever offered).
export interface ActionViewModel {
  id: string; state: string; digest: string; canApprove: boolean; canExecute: boolean
}
export function actionControls(a: ActionViewModel) {
  return {
    approve: a.state === 'awaiting_approval' && a.canApprove,
    execute: a.state === 'authorized' && a.canExecute,
    retry: false,
  }
}
