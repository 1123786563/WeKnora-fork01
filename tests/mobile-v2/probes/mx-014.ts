// MX-014 probe · 分页稳定性与筛选迟到响应观察器
// frozen 场景：equal-updated-at-two-pages × filter-change-late-response。
// 真实 createSessionListController + 真实 generation 守卫；脚本化 loadPage 只替换网络边界：
// 1) 相同 updated_at 跨两页（id 决胜翻页）无重复；
// 2) 筛选变更后旧筛选的迟到响应被丢弃（appliedFilter 保持 waiting_user）。
import { createSessionListController, type ListPageRequest, type SessionListItem } from '../../../packages/domain/src/mobile/session-list.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  duplicateIds: string[];
  appliedFilter: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'equal-updated-at-two-pages' || input.fault !== 'filter-change-late-response') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const calls: ListPageRequest[] = [];
  let generation = 1;
  const pageOne: SessionListItem[] = [
    { runId: 'run-3', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'running' },
    { runId: 'run-2', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'waiting_user' },
  ];
  const pageTwo: SessionListItem[] = [
    { runId: 'run-1', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'running' },
  ];
  // waiting_user 两页：全部同 updated_at——翻页必须由 (updated_at, runId) cursor 决胜
  const waitingUserPage: SessionListItem[] = [
    { runId: 'run-20', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'waiting_user' },
    { runId: 'run-19', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'waiting_user' },
  ];
  const waitingUserPageTwo: SessionListItem[] = [
    { runId: 'run-18', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'waiting_user' },
    // 重放上一页条目（重复页）：必须被 id 去重
    { runId: 'run-20', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'waiting_user' },
  ];

  const controller = createSessionListController({
    loadPage: async (request) => {
      calls.push(request);
      if (request.filter === 'waiting_user') {
        // 首页带 nextCursor（相同 updated_at，runId 决胜）；第二页含重放条目
        if (request.after === undefined) {
          return { items: waitingUserPage, nextCursor: { updatedAt: '2026-09-18T08:00:00Z', runId: 'run-19' } };
        }
        return { items: waitingUserPageTwo };
      }
      if (request.after === undefined) {
        // 首页延迟返回（制造「旧筛选迟到」时序）
        await new Promise((resolve) => setTimeout(resolve, 5));
        return { items: pageOne, nextCursor: { updatedAt: '2026-09-18T08:00:00Z', runId: 'run-2' } };
      }
      return { items: pageTwo };
    },
    capture: () => ({ generation }),
    accept: (candidate) => candidate === generation,
    now: () => 0,
    debounceMs: 0,
  });

  // 触发首载（filter=all，慢响应在途）……
  const firstLoad = controller.refresh();
  // ……立即切筛选（查询身份变更）
  await controller.setFilter('waiting_user');
  await firstLoad;
  // 再翻一页（同 updated_at，id 决胜）
  await controller.loadNextPage();

  // 真实翻页断言（R1 P2）：waiting_user 两页（同 updated_at）+ 重放条目
  const pagedState = controller.state;
  const pagedIds = pagedState.items.map((item) => item.runId);
  if (pagedState.items.length !== 3 || new Set(pagedIds).size !== 3) {
    throw new Error(`equal-timestamp paging must apply 3 unique items, got ${JSON.stringify(pagedIds)}`);
  }

  // 空间切换维度（R1 P2-1）：代际前进后，旧空间的在途响应不得落地
  let staleVisible = false;
  const staleLoad = (async () => {
    const stale = createSessionListController({
      loadPage: async () => {
        await new Promise((resolve) => setTimeout(resolve, 10));
        return { items: [{ runId: 'stale-run', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'running' }] };
      },
      capture: () => ({ generation }),
      accept: (candidate) => candidate === generation,
      now: () => 0,
      debounceMs: 0,
    });
    const inFlight = stale.refresh();
    generation += 1; // 空间切换：代际前进
    await inFlight;
    staleVisible = stale.state.items.some((item) => item.runId === 'stale-run');
  })();
  await staleLoad;
  if (staleVisible) throw new Error('response captured before a scope switch must not land after it');

  // 空间切换失效（R1 P2-2）：已落地 items/nextCursor 必须可失效
  if (controller.state.items.length === 0) throw new Error('precondition: landed items exist');
  controller.invalidate();
  if (controller.state.items.length !== 0 || controller.state.nextCursor !== undefined) {
    throw new Error('invalidate must clear landed items and cursor');
  }

  const state = controller.state;
  if (calls.some((call) => call.filter === 'waiting_user' && call.after === undefined) === false) {
    throw new Error('filter change must issue a fresh first page query');
  }
  // 旧 all 首页若被应用，items 会含 running 条目——appliedFilter 语义以最终 state 为准
  if (state.items.some((item) => item.runStatus === 'running')) {
    throw new Error('late response from the previous filter must be dropped');
  }
  return {
    duplicateIds: controller.duplicateIds(),
    appliedFilter: state.filter,
  };
}
