import type { WeKnoraClient } from '@weknora/api-client';
import { ActionApproval, type ActionPrepareContext } from './ActionApproval.tsx';
import { TaskBudget, type TaskBudgetSnapshot } from '../commercial/TaskBudget.tsx';

interface TaskDetailCommercePanelProps {
  client: WeKnoraClient;
  taskId: string;
  actionId: string;
  /** Connection context so edited action content can be re-prepared with a new digest. */
  prepare?: ActionPrepareContext;
  /** Task budget facts; the readout shows no fabricated numbers when absent. */
  budget?: TaskBudgetSnapshot;
}

/**
 * TaskDetailCommercePanel composes the task budget and the external action
 * approval at one task-detail access point. The two concerns are kept in
 * SEPARATE components with SEPARATE consent controls on purpose: raising a
 * budget and approving one exact external write are never one click.
 *
 * Mount note (honest disclosure): this React seam currently renders single
 * pages (KnowledgeBasesPage and its siblings) with no task-detail route of
 * its own yet, so the panel is exported for the task-detail host (and the
 * Craft/mobile consumers share the same contracts/client) instead of being
 * wired into a navigation that does not exist here.
 */
export function TaskDetailCommercePanel({ client, taskId, actionId, prepare, budget }: TaskDetailCommercePanelProps) {
  return (
    <section aria-label="任务预算与外部写操作审批">
      <TaskBudget client={client} taskId={taskId} snapshot={budget} />
      <ActionApproval client={client} actionId={actionId} prepare={prepare} />
    </section>
  );
}
