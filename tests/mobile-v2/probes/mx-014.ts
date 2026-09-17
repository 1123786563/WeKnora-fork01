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
  const waitingUserPage: SessionListItem[] = [
    { runId: 'run-2', title: '', updatedAt: '2026-09-18T08:00:00Z', runStatus: 'waiting_user' },
  ];

  const controller = createSessionListController({
    loadPage: async (request) => {
      calls.push(request);
      if (request.filter === 'waiting_user') {
        // 旧查询（all）的迟到响应：等一拍后返回全量页——必须被丢弃
        await Promise.resolve();
        return { items: waitingUserPage };
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
  // 空间切换代际（迟到响应兜底路径）
  generation += 1;

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
