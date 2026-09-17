import { createGenerationGuard } from './query-scope.ts';

/**
 * 会话列表分页/搜索/筛选控制器（MX-014）。
 * 冻结规则：
 * - 稳定分页游标 = (updated_at, run_id) 二元组——相同 updated_at 也能正确翻页（id 决胜）；
 * - 筛选条件纳入查询身份：filter/status/search 变更即新查询（旧查询的迟到响应经 scope
 *   generation 守卫丢弃，不得写入新结果）；
 * - 搜索去抖只控制读请求（不吞用户输入、不触发写）；
 * - 条目按 id 去重（任何重放/重复页不产生 duplicateIds）；
 * - 切空间由 scope generation 前进自动失效（MX-011 语义，无需控制器额外处理）。
 */

export interface SessionListItem {
  runId: string;
  title: string;
  updatedAt: string;
  runStatus: string;
}

export type SessionFilter = 'all' | 'running' | 'waiting_user' | 'terminal';

export interface ListPageCursor {
  updatedAt: string;
  runId: string;
}

export interface ListPageRequest {
  filter: SessionFilter;
  search: string;
  after?: ListPageCursor;
  pageSize: number;
}

export interface ListPageResponse {
  items: SessionListItem[];
  nextCursor?: ListPageCursor;
}

export interface SessionListPorts {
  loadPage(request: ListPageRequest): Promise<ListPageResponse>;
  /** scope acceptance：generation 失效即丢弃（真实 ProductScope.capture/accept 注入） */
  capture(): { generation: number };
  accept(generation: number): boolean;
  now(): number;
  /** 读去抖（默认 250ms；测试可注 0） */
  debounceMs?: number;
}

export interface SessionListState {
  items: SessionListItem[];
  filter: SessionFilter;
  search: string;
  loading: boolean;
  nextCursor?: ListPageCursor;
  exhausted: boolean;
}

export function createSessionListController(ports: SessionListPorts) {
  const debounceMs = ports.debounceMs ?? 250;
  let state: SessionListState = { items: [], filter: 'all', search: '', loading: false, exhausted: false };
  let searchTimer: ReturnType<typeof setTimeout> | undefined;
  let queryEpoch = 0;
  const listeners = new Set<() => void>();
  function emit(): void {
    for (const listener of listeners) listener();
  }

  function dedup(items: SessionListItem[]): SessionListItem[] {
    const seen = new Set<string>();
    const out: SessionListItem[] = [];
    for (const item of [...items].sort((a, b) => (a.updatedAt === b.updatedAt ? (a.runId < b.runId ? 1 : -1) : a.updatedAt < b.updatedAt ? 1 : -1))) {
      if (seen.has(item.runId)) continue;
      seen.add(item.runId);
      out.push(item);
    }
    return out;
  }

  async function fetchPage(reset: boolean): Promise<void> {
    const epoch = ++queryEpoch;
    const handle = ports.capture();
    const guard = createGenerationGuard((generation) => ports.accept(generation), handle.generation);
    const request: ListPageRequest = {
      filter: state.filter,
      search: state.search,
      pageSize: 20,
      ...(reset || !state.nextCursor ? {} : { after: state.nextCursor }),
    };
    state = { ...state, loading: true };
    emit();
    let response: ListPageResponse;
    try {
      response = await ports.loadPage(request);
    } finally {
      if (epoch === queryEpoch) state = { ...state, loading: false };
    }
    // 迟到响应：查询身份已变（新 filter/search/翻页）或空间已切换 → 丢弃
    if (epoch !== queryEpoch || !guard.isCurrent()) return;
    state = {
      ...state,
      items: dedup(reset ? response.items : [...state.items, ...response.items]),
      nextCursor: response.nextCursor,
      exhausted: response.nextCursor === undefined,
    };
    emit();
  }

  return {
    get state(): SessionListState {
      return state;
    },
    /** 首载/重置（filter 与 search 已是当前值） */
    async refresh(): Promise<void> {
      return fetchPage(true);
    },
    async loadNextPage(): Promise<void> {
      if (state.loading || state.exhausted) return;
      return fetchPage(false);
    },
    /** 筛选立即换查询身份（不等去抖） */
    async setFilter(filter: SessionFilter): Promise<void> {
      if (state.filter === filter) return;
      state = { ...state, filter };
      return fetchPage(true);
    },
    /** 搜索仅去抖读请求；输入本身即时保存于 state */
    setSearch(search: string): void {
      state = { ...state, search };
      if (searchTimer !== undefined) clearTimeout(searchTimer);
      searchTimer = setTimeout(() => {
        void fetchPage(true);
      }, debounceMs);
    },
    /**
     * 空间切换失效（P2-2）：清除已落地 items/nextCursor 并作废一切在途查询。
     * 接线合同：宿主在 scope identity 变化时必须调用本方法（或按 scope 重建实例）——
     * generation 守卫只拦在途响应，不清已落地状态。
     */
    invalidate(): void {
      queryEpoch += 1;
      if (searchTimer !== undefined) clearTimeout(searchTimer);
      state = { items: [], filter: state.filter, search: state.search, loading: false, exhausted: false };
      emit();
    },
    /** 状态订阅（替代轮询重绘）。返回取消函数。 */
    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    /** 释放（清理去抖定时器；卸载后不再触发读）。 */
    dispose(): void {
      queryEpoch += 1;
      if (searchTimer !== undefined) clearTimeout(searchTimer);
      listeners.clear();
    },
    /** 重复键观测（列表稳定 id 合同） */
    duplicateIds(): string[] {
      const seen = new Set<string>();
      const dup: string[] = [];
      for (const item of state.items) {
        if (seen.has(item.runId)) dup.push(item.runId);
        seen.add(item.runId);
      }
      return dup;
    },
  };
}

export type SessionListController = ReturnType<typeof createSessionListController>;
