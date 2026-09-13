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
