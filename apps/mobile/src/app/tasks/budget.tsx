import { useEffect, useRef, useState } from 'react';
import { useLocalSearchParams } from 'expo-router';
import { TaskBudgetScreen } from '../../screens/TaskBudgetScreen.tsx';
import { createTaskBudgetController, type TaskBudgetController, type TaskBudgetViewState } from '../../task-budget-view.ts';
import { activeTaskOffice } from '../../composition.ts';

/** /tasks/budget?taskId=.. 挂载生命周期宿主：控制器在 effect 内创建、卸载即 dispose——
 * 与 /tasks/detail 同一模式；只消费 Task Office Interface（module-seams §5.2/§10）。 */
export function TaskBudgetRouteLifecycle({ taskId }: { taskId: string }) {
  const [state, setState] = useState<TaskBudgetViewState>({ loading: true, extending: false });
  const [credits, setCredits] = useState('100');
  const [confirmed, setConfirmed] = useState(false);
  const controllerRef = useRef<TaskBudgetController | undefined>(undefined);
  useEffect(() => {
    const office = activeTaskOffice();
    if (office === undefined || taskId === '') {
      setState({ loading: false, extending: false, error: '请先登录并激活空间，再查看任务预算。' });
      return;
    }
    const controller = createTaskBudgetController(office, { taskId });
    controllerRef.current = controller;
    setState(controller.state());
    const unsubscribe = controller.subscribe(setState);
    void controller.refresh();
    return () => {
      unsubscribe();
      controller.dispose();
      controllerRef.current = undefined;
    };
  }, [taskId]);
  return (
    <TaskBudgetScreen
      taskId={taskId}
      state={state}
      confirmed={confirmed}
      credits={credits}
      onToggleConfirmed={setConfirmed}
      onChangeCredits={setCredits}
      onRefresh={() => { void controllerRef.current?.refresh(); }}
      onExtend={() => { void controllerRef.current?.extend(Number(credits)); }}
    />
  );
}

/** Expo Router 文件路由：/tasks/budget?taskId=..。 */
export default function TaskBudgetRoute() {
  const params = useLocalSearchParams<{ taskId?: string }>();
  return <TaskBudgetRouteLifecycle taskId={String(params.taskId ?? '')} />;
}
