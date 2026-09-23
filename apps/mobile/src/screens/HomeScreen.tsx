import { useEffect, useState } from 'react';
import { Button, Text, View } from 'react-native';
import { router } from 'expo-router';
import type { HomeView, TaskOffice } from '@weknora/mobile-core';

export interface HomeScreenProps {
  deploymentLabel: string;
  tenants: Array<{ id: string; name?: string }>;
  activeTenantId: string;
  onActivateTenant(tenantId: string): void;
  onSignOut(): Promise<void>;
  taskOffice: TaskOffice;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** Home 一级入口：三段聚合视图（module-seams §5.2 home(query)），只消费 Task Office 视图。 */
export function HomeScreen({ deploymentLabel, tenants, activeTenantId, onActivateTenant, onSignOut, taskOffice }: HomeScreenProps) {
  const [view, setView] = useState<HomeView | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const load = (): void => {
    setLoading(true);
    setError(undefined);
    void taskOffice.home().then(setView, (failure: unknown) => setError(errorMessage(failure))).finally(() => setLoading(false));
  };
  useEffect(load, []);
  return (
    <View>
      <Text>{deploymentLabel}</Text>
      {tenants.length > 1
        ? tenants.map((tenant) => (
            <Button key={tenant.id} title={tenant.name ?? tenant.id} onPress={() => onActivateTenant(tenant.id)} />
          ))
        : <Text>{tenants[0]?.name ?? activeTenantId}</Text>}
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
      <Button title="View all tasks" onPress={() => router.push('/tasks')} />
      {loading && <Text>Loading</Text>}
      {error !== undefined && (
        <View>
          <Text>{error}</Text>
          <Button title="Retry" onPress={load} />
        </View>
      )}
      {error === undefined && !loading && view !== undefined && (
        <View>
          <Text>{`Needs me (${view.needsMe.length})`}</Text>
          <Text>{`Unread ${view.unreadNotifications}`}</Text>
          {view.needsMe.map((item) => (
            <Text key={item.interactionId}>{`${item.kind} · ${item.createdAt}`}</Text>
          ))}
          {view.needsMe.length === 0 && view.unreadNotifications === 0 && <Text>Nothing needs you</Text>}
          <Text>{`Running (${view.running.length})`}</Text>
          {view.running.map((card) => (
            <Text key={card.runId}>{`${card.title || card.taskId} · ${card.runStatus}${card.attention === 'required' ? ' · needs you' : ''}`}</Text>
          ))}
          <Text>{`Recently completed (${view.recentlyCompleted.length})`}</Text>
          {view.recentlyCompleted.map((card) => (
            <Text key={card.runId}>{`${card.title || card.taskId} · ${card.runStatus}`}</Text>
          ))}
        </View>
      )}
      {/* 显式装载/重试控制：与测试桩（no-op useEffect）和真机刷新共用同一路径。 */}
      <Button title="Load home" onPress={load} />
    </View>
  );
}
