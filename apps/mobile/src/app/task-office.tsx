import { useEffect, useState, useSyncExternalStore } from 'react';
import { router } from 'expo-router';
import { Text } from 'react-native';
import type { Deployment, MobileRuntime, RuntimeSnapshot, ScopeLease, TaskCard, TaskListPage } from '@weknora/mobile-core';
import { activeMobileRuntime, activeTaskOffice, deploymentScopeKey } from '../composition.ts';
import { createAuthorizedTaskEntry, detectNativeTaskCapabilities } from '../task-office/native-boundary/task-entry.ts';
import { TaskEntryScreen, type ExistingTaskRow } from '../task-office/native-boundary/TaskEntryScreen.tsx';

function taskRow(card: TaskCard): ExistingTaskRow {
  return { taskId: card.taskId, runId: card.runId, title: card.title, runStatus: card.runStatus, updatedAt: card.updatedAt };
}

type AuthorizedRuntimeSnapshot = RuntimeSnapshot & { deployment: Deployment; identity: { userId: string; activeTenantId: string } };

function authorizedSnapshot(snapshot: RuntimeSnapshot): snapshot is AuthorizedRuntimeSnapshot {
  return snapshot.surface === 'authorized' && snapshot.deployment !== undefined && snapshot.identity?.userId !== undefined && snapshot.identity.activeTenantId !== undefined;
}

export function TaskOfficeEntryLifecycle({ runtime, origin, tenantId, openingLease }: {
  runtime: MobileRuntime;
  origin: string;
  tenantId: string;
  openingLease: ScopeLease;
}) {
  const [tasks, setTasks] = useState<ExistingTaskRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>(undefined);

  const loadExisting = async (isCurrent: () => boolean = () => true): Promise<void> => {
    setLoading(true);
    setError(undefined);
    const entry = createAuthorizedTaskEntry({ current: () => {
      const snapshot = runtime.snapshot();
      const lease = runtime.scopeLease();
      return { surface: snapshot.surface, hasScopeLease: lease !== undefined && lease === openingLease, office: activeTaskOffice() };
    } });
    try {
      const page: TaskListPage = await entry.listExisting();
      if (!isCurrent() || runtime.scopeLease() !== openingLease) return;
      setTasks(page.items.map(taskRow));
    } catch (failure) {
      if (!isCurrent() || runtime.scopeLease() !== openingLease) return;
      setTasks([]);
      setError(failure instanceof Error ? failure.message : String(failure));
    } finally {
      if (isCurrent() && runtime.scopeLease() === openingLease) setLoading(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    void loadExisting(() => !cancelled);
    return () => { cancelled = true; };
    // The parent keys this route by deployment + tenant and supplies a new lease when Runtime rotates it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runtime, origin, tenantId, openingLease]);

  const openTask = (task: ExistingTaskRow): void => {
    const snapshot = runtime.snapshot();
    if (!authorizedSnapshot(snapshot) || runtime.scopeLease() !== openingLease) {
      setTasks([]);
      setError('工作空间已切换，请返回任务列表后重新打开。');
      return;
    }
    router.push({ pathname: '/tasks/detail', params: { taskId: task.taskId, runId: task.runId } });
  };

  return <TaskEntryScreen loading={loading} tasks={tasks} error={error} capabilities={detectNativeTaskCapabilities()} onRefresh={() => { void loadExisting(); }} onOpenTask={openTask} />;
}

/** Authenticated existing-Task entry. Runtime changes remount it by deployment origin and tenant. */
export default function TaskOfficeEntryRoute() {
  const runtime = activeMobileRuntime();
  const snapshot = useSyncExternalStore(runtime.subscribe, runtime.snapshot, runtime.snapshot);
  const openingLease = runtime.scopeLease();
  if (!authorizedSnapshot(snapshot) || openingLease === undefined) {
    return <Text accessibilityRole="text">请先登录并激活空间，再查看已授权任务。</Text>;
  }
  const origin = snapshot.deployment.origin;
  const tenantId = snapshot.identity.activeTenantId;
  return <TaskOfficeEntryLifecycle key={deploymentScopeKey(origin, tenantId)} runtime={runtime} origin={origin} tenantId={tenantId} openingLease={openingLease} />;
}
