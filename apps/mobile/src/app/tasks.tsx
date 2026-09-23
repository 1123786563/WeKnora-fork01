import { router } from 'expo-router';
import { MobileTasks } from '../composition.ts';

/** /tasks 一级入口；Surface 仍由 Runtime 快照裁决，行点击进入 /tasks/detail。 */
export default function Tasks() {
  return <MobileTasks onOpenTask={(taskId, runId) => router.push({ pathname: '/tasks/detail', params: { taskId, runId } })} />;
}
