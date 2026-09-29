import { useEffect, useState, useSyncExternalStore } from 'react';
import { router } from 'expo-router';
import { Text } from 'react-native';
import type { Deployment, MobileRuntime, RuntimeSnapshot, ScopeLease } from '@weknora/mobile-core';
import { activeMobileRuntime, activeTaskOffice, deploymentScopeKey } from '../composition.ts';
import { createAuthorizedTaskEntry, detectNativeTaskCapabilities } from '../task-office/native-boundary/task-entry.ts';
import { TaskEntryScreen, type ExistingTaskRow } from '../task-office/native-boundary/TaskEntryScreen.tsx';
import { createTaskOfficeListController, taskOfficeLifecycleKey } from './task-office-state.ts';

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

  const [controller] = useState(() => createTaskOfficeListController({
    openingLease,
    publish: (next) => { setTasks(next.tasks); setLoading(next.loading); setError(next.error); },
    read: async () => {
    const entry = createAuthorizedTaskEntry({ current: () => {
      const snapshot = runtime.snapshot();
      const lease = runtime.scopeLease();
      return { surface: snapshot.surface, hasScopeLease: lease !== undefined && lease === openingLease, office: activeTaskOffice() };
    } });
      return entry.listExisting();
    },
  }));

  useEffect(() => {
    void controller.load();
    return () => controller.dispose();
    // The parent keys this route by deployment, tenant, user, and lease.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [controller]);

  const openTask = (task: ExistingTaskRow): void => {
    const snapshot = runtime.snapshot();
    if (!authorizedSnapshot(snapshot) || runtime.scopeLease() !== openingLease) {
      setTasks([]);
      setError('工作空间已切换，请返回任务列表后重新打开。');
      return;
    }
    router.push({ pathname: '/tasks/detail', params: { taskId: task.taskId, runId: task.runId } });
  };

  return <TaskEntryScreen loading={loading} tasks={tasks} error={error} capabilities={detectNativeTaskCapabilities()} onRefresh={() => { void controller.load(); }} onOpenTask={openTask} />;
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
  const userId = snapshot.identity.userId;
  return <TaskOfficeEntryLifecycle key={taskOfficeLifecycleKey(origin, tenantId, userId, openingLease)} runtime={runtime} origin={origin} tenantId={tenantId} openingLease={openingLease} />;
}
