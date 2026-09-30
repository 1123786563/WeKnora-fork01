import { useEffect, useState } from 'react';
import { Button, Text, TextInput, View } from 'react-native';
import type { TaskCard, TaskListPage, TaskOffice, TaskStatusFilter } from '@weknora/mobile-core';

export interface TasksScreenProps {
  taskOffice: TaskOffice;
  onOpenTask?: (card: TaskCard) => void;
  onOpenLegacy?: () => void;
  onOpenTaskOffice?: () => void;
}

const STATUS_FILTERS: Array<{ id: '' | TaskStatusFilter; label: string }> = [
  { id: '', label: 'All' },
  { id: 'running', label: 'Running' },
  { id: 'waiting_user', label: 'Waiting' },
  { id: 'succeeded', label: 'Done' },
  { id: 'failed', label: 'Failed' },
];

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** Tasks 一级入口：搜索/筛选/归档与翻页；cursor 与查询身份归 Task Office，本屏只持有渲染态。 */
export function TasksScreen({ taskOffice, onOpenTask, onOpenLegacy, onOpenTaskOffice }: TasksScreenProps) {
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<'' | TaskStatusFilter>('');
  const [archived, setArchived] = useState(false);
  const [items, setItems] = useState<TaskCard[]>([]);
  const [duplicateRunIds, setDuplicateRunIds] = useState<string[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const reload = (): void => {
    setLoading(true);
    setError(undefined);
    void taskOffice.tasks({ ...(search === '' ? {} : { search }), ...(status === '' ? {} : { status }), ...(archived ? { archived: true } : {}) })
      .then((page: TaskListPage) => {
        setItems(page.items);
        setDuplicateRunIds(page.duplicateRunIds);
        setHasMore(page.nextCursor !== undefined);
      }, (failure: unknown) => setError(errorMessage(failure)))
      .finally(() => setLoading(false));
  };
  useEffect(reload, []);

  const loadMore = (): void => {
    setLoading(true);
    void taskOffice.moreTasks()
      .then((page: TaskListPage) => {
        setItems((current) => [...current, ...page.items]);
        setDuplicateRunIds(page.duplicateRunIds);
        setHasMore(page.nextCursor !== undefined);
      }, (failure: unknown) => setError(errorMessage(failure)))
      .finally(() => setLoading(false));
  };

  const archive = (taskId: string, restore: boolean): void => {
    setLoading(true);
    void (restore ? taskOffice.restore(taskId) : taskOffice.archive(taskId))
      .then(reload, (failure: unknown) => setError(errorMessage(failure)))
      .finally(() => setLoading(false));
  };

  return (
    <View>
      <Text>Tasks</Text>
      {onOpenTaskOffice !== undefined && <Button title="Task Office" onPress={onOpenTaskOffice} />}
      <TextInput value={search} onChangeText={setSearch} placeholder="Search tasks" />
      {STATUS_FILTERS.map((filter) => (
        <Button key={filter.id} title={filter.label} onPress={() => { setStatus(filter.id); }} />
      ))}
      <Button title={archived ? 'Showing archived' : 'Showing active'} onPress={() => { setArchived(!archived); }} />
      <Button title="Search" onPress={reload} />
      {loading && <Text>Loading</Text>}
      {error !== undefined && <Text>{error}</Text>}
      {duplicateRunIds.length > 0 && <Text>{`Duplicated keys observed: ${duplicateRunIds.join(', ')}`}</Text>}
      {items.length === 0 && error === undefined && !loading && <Text>No tasks yet</Text>}
      {items.map((card) => (
        <View key={card.runId}>
          <Text>{`${card.title || card.taskId} · ${card.runStatus}${card.attention === 'required' ? ' · needs you' : ''}`}</Text>
          <Button title={archived ? 'Restore' : 'Archive'} onPress={() => { archive(card.taskId, archived); }} />
          {onOpenTask !== undefined && <Button title="Details" onPress={() => onOpenTask(card)} />}
        </View>
      ))}
      {hasMore && <Button title="Load more" onPress={loadMore} />}
      {onOpenLegacy !== undefined && <Button title="Legacy tasks" onPress={onOpenLegacy} />}
    </View>
  );
}
