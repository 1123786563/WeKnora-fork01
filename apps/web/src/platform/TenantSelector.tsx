// F2 — 超管侧栏空间切换器（Vue components/TenantSelector.vue 同构平移）。
//
// 挂载门控与 Vue menu.vue:53 一致：canAccessAllTenants 且侧栏展开时渲染
// （本组件不含门控，由 PlatformShell 决定）。行为对齐：
//   • 触发卡：当前空间名 + swap 图标；
//   • 下拉：标题「切换空间」+ 搜索框（300ms 防抖，纯数字同时作为空间 ID
//     精确匹配）+ 分页列表（page_size 20，滚动到底加载更多）+ 选中 check；
//   • 数据源 GET /api/v1/tenants/search（跨空间中间件，仅超管可达）；
//   • 「+ 创建新空间」入口由宿主提供（shell 复用既有 CreateTenantDialog）。
// 文案：packages/i18n 未收录 tenant.currentTenant/… 键，按 global-file-drop.ts
// 先例本地维护 Record<Locale, …>（值逐字取自 frontend i18n locales）。
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';
import { Icon as TIcon } from 'tdesign-icons-react';
import { MessagePlugin } from 'tdesign-react';

export interface TenantSelectorTenant {
  id: number;
  name: string;
}

export interface TenantSelectorPage {
  items: TenantSelectorTenant[];
  total: number;
}

interface TenantSelectorCopy {
  currentTenant: string;
  switchTenant: string;
  unknown: string;
  searchPlaceholder: string;
  noMatch: string;
  loadTenantsFailed: string;
  loading: string;
}

const TENANT_SELECTOR_COPY: Record<Locale, TenantSelectorCopy> = {
  'zh-CN': { currentTenant: '当前空间', switchTenant: '切换空间', unknown: '未知', searchPlaceholder: '搜索空间名称或输入空间 ID...', noMatch: '未找到匹配的空间', loadTenantsFailed: '加载空间列表失败', loading: '加载中...' },
  'en-US': { currentTenant: 'Current Workspace', switchTenant: 'Switch workspace', unknown: 'Unknown', searchPlaceholder: 'Search by name or enter workspace ID...', noMatch: 'No matching workspaces found', loadTenantsFailed: 'Failed to load workspace list', loading: 'Loading...' },
  'ja-JP': { currentTenant: '現在のワークスペース', switchTenant: 'ワークスペースを切り替え', unknown: '不明', searchPlaceholder: '名前で検索、またはワークスペースIDを入力...', noMatch: '一致するワークスペースが見つかりません', loadTenantsFailed: 'ワークスペース一覧の読み込みに失敗しました', loading: '読み込み中...' },
  'ko-KR': { currentTenant: '현재 워크스페이스', switchTenant: '워크스페이스 전환', unknown: '알 수 없음', searchPlaceholder: '워크스페이스 이름 검색 또는 워크스페이스 ID 입력...', noMatch: '일치하는 워크스페이스를 찾을 수 없습니다', loadTenantsFailed: '워크스페이스 목록 로드 실패', loading: '로딩 중...' },
  'ru-RU': { currentTenant: 'Текущее пространство', switchTenant: 'Переключить рабочее пространство', unknown: 'Неизвестно', searchPlaceholder: 'Поиск по имени или введите ID пространства...', noMatch: 'Не найдено подходящих пространств', loadTenantsFailed: 'Не удалось загрузить список пространств', loading: 'Загрузка...' },
};

export const TENANT_SELECTOR_PAGE_SIZE = 20;
export const TENANT_SEARCH_DEBOUNCE_MS = 300;

/** Vue searchTenants：纯数字关键词同时作为 tenant_id（精确）与 keyword（模糊）。 */
export function tenantSearchPath(keyword: string, page: number, pageSize: number): string {
  const params = new URLSearchParams();
  const trimmed = keyword.trim();
  if (trimmed) params.set('keyword', trimmed);
  if (trimmed && /^\d+$/.test(trimmed)) params.set('tenant_id', trimmed);
  params.set('page', String(page));
  params.set('page_size', String(pageSize));
  return `/api/v1/tenants/search?${params.toString()}`;
}

/** 解析 GET /api/v1/tenants/search 响应（{success, data:{items,total}} 宽松解析）。 */
export function parseTenantSearchPage(payload: unknown): TenantSelectorPage {
  const root = payload as { success?: unknown; data?: unknown } | null;
  const data = root?.data as { items?: unknown; total?: unknown } | null | undefined;
  const rawItems = Array.isArray(data?.items) ? (data?.items as unknown[]) : [];
  const items = rawItems.flatMap((item) => {
    if (item === null || typeof item !== 'object') return [];
    const row = item as { id?: unknown; name?: unknown };
    if (row.id === undefined || row.id === null) return [];
    return [{ id: Number(row.id), name: typeof row.name === 'string' && row.name ? row.name : `#${String(row.id)}` }];
  });
  const total = typeof data?.total === 'number' && Number.isFinite(data.total) ? Math.floor(data.total) : items.length;
  return { items, total };
}

export function hasMoreTenantPages(loaded: number, total: number): boolean {
  return loaded < total;
}

export interface TenantSelectorProps {
  locale: Locale;
  /** GET /api/v1/tenants/search 通道（client.request 同签名）。 */
  request: (input: { method: 'GET'; path: string }) => Promise<unknown>;
  currentTenantName: string;
  currentTenantId: string | null;
  canCreateTenant: boolean;
  /** 切换进行中（列表项禁用），由宿主管理（shell switchTenant 的 pending）。 */
  switchPending: boolean;
  onSelectTenant: (tenantId: string, tenantName: string) => void;
  /** 「+ 创建新空间」入口（宿主打开 CreateTenantDialog）。 */
  onCreateTenant?: () => void;
}

export function TenantSelector({ locale, request, currentTenantName, currentTenantId, canCreateTenant, switchPending, onSelectTenant, onCreateTenant }: TenantSelectorProps): ReactNode {
  const copy = TENANT_SELECTOR_COPY[locale];
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [tenants, setTenants] = useState<TenantSelectorTenant[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const tenantListRef = useRef<HTMLDivElement | null>(null);
  const loadingRef = useRef(false);
  const searchTimer = useRef<number | null>(null);
  useEffect(() => () => { if (searchTimer.current !== null) window.clearTimeout(searchTimer.current); }, []);

  // Vue loadTenants：keyword/tenant_id/page 分页拉取，失败 MessagePlugin.error。
  const loadTenants = useCallback(async (nextPage: number, keyword: string, append: boolean) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    try {
      const payload = await request({ method: 'GET', path: tenantSearchPath(keyword, nextPage, TENANT_SELECTOR_PAGE_SIZE) });
      const parsed = parseTenantSearchPage(payload);
      setTenants((current) => append ? [...current, ...parsed.items.filter((item) => !current.some((row) => row.id === item.id))] : parsed.items);
      setTotal(parsed.total);
      setPage(nextPage);
    } catch {
      void MessagePlugin.error(copy.loadTenantsFailed);
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, [copy.loadTenantsFailed, request]);

  const closeDropdown = useCallback(() => {
    setOpen(false);
    setQuery('');
    if (searchTimer.current !== null) {
      window.clearTimeout(searchTimer.current);
      searchTimer.current = null;
    }
  }, []);

  const toggleDropdown = useCallback(() => {
    setOpen((current) => {
      const next = !current;
      if (next && tenants.length === 0) void loadTenants(1, '', false);
      if (next) window.setTimeout(() => searchInputRef.current?.focus(), 0);
      return next;
    });
  }, [loadTenants, tenants.length]);

  // Vue handleSearchInput：300ms 防抖后从第一页重拉。
  const handleSearchInput = useCallback((value: string) => {
    setQuery(value);
    if (searchTimer.current !== null) window.clearTimeout(searchTimer.current);
    searchTimer.current = window.setTimeout(() => {
      void loadTenants(1, value, false);
    }, TENANT_SEARCH_DEBOUNCE_MS);
  }, [loadTenants]);

  // Vue handleScroll：距底 50px 内加载下一页。
  const handleScroll = useCallback(() => {
    const element = tenantListRef.current;
    if (!element) return;
    const nearBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 50;
    if (nearBottom && hasMoreTenantPages(tenants.length, total) && !loadingRef.current) {
      void loadTenants(page + 1, query, true);
    }
  }, [loadTenants, page, query, tenants.length, total]);

  const selectTenant = useCallback((tenant: TenantSelectorTenant) => {
    closeDropdown();
    onSelectTenant(String(tenant.id), tenant.name);
  }, [closeDropdown, onSelectTenant]);

  const isSelected = (tenantId: number): boolean => currentTenantId !== null && String(tenantId) === currentTenantId;

  return (
    <div className="tenant-selector" data-testid="tenant-selector">
      <div className="tenant-trigger" role="button" tabIndex={0} aria-haspopup="menu" aria-expanded={open}
        onClick={toggleDropdown}
        onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); toggleDropdown(); } }}>
        <div className="tenant-info">
          <div className="tenant-label">{copy.currentTenant}</div>
          <div className="tenant-name-row">
            <span className="tenant-name">{currentTenantName || copy.unknown}</span>
            <TIcon name="swap" className="tenant-switch-icon" />
          </div>
        </div>
      </div>

      {open ? <>
        <div className="tenant-dropdown" role="menu" aria-label={copy.switchTenant} onClick={(event) => event.stopPropagation()}>
          <div className="dropdown-header">
            <span className="dropdown-title">{copy.switchTenant}</span>
            <div className="search-box">
              <TIcon name="search" className="search-icon" />
              <input ref={searchInputRef} value={query} type="text" placeholder={copy.searchPlaceholder} className="search-input"
                aria-label={copy.searchPlaceholder}
                onChange={(event) => handleSearchInput(event.currentTarget.value)}
                onKeyDown={(event) => { if (event.key === 'Escape') closeDropdown(); }} />
              {query ? <TIcon name="close-circle-filled" className="clear-icon" onClick={() => { setQuery(''); void loadTenants(1, '', false); }} /> : null}
            </div>
          </div>

          <div className="tenant-list" ref={tenantListRef} onScroll={handleScroll}>
            {loading && tenants.length === 0 ? <div className="tenant-loading" role="status">{copy.loading}</div> : null}
            {tenants.length > 0 ? tenants.map((tenant) => (
              <div key={tenant.id} role="menuitem" aria-selected={isSelected(tenant.id)}
                className={'tenant-item' + (isSelected(tenant.id) ? ' selected' : '')}
                onClick={() => { if (!switchPending) selectTenant(tenant); }}>
                <div className="tenant-item-content">
                  <div className={'tenant-item-avatar' + (isSelected(tenant.id) ? ' active' : '')}>{tenant.name.charAt(0).toUpperCase()}</div>
                  <div className="tenant-item-info">
                    <span className="tenant-item-name">{tenant.name}</span>
                    <span className="tenant-item-id">ID: {tenant.id}</span>
                  </div>
                </div>
                {isSelected(tenant.id) ? <TIcon name="check" size="16px" className="check-icon" /> : null}
              </div>
            )) : !loading ? <div className="tenant-empty">{copy.noMatch}</div> : null}
            {loading && tenants.length > 0 ? <div className="tenant-loading-more" role="status">{copy.loading}</div> : null}
          </div>

          {canCreateTenant ? (
            <div className="tenant-create-action" role="button" tabIndex={0}
              onClick={() => { closeDropdown(); onCreateTenant?.(); }}
              onKeyDown={(event) => { if (event.key === 'Enter') { closeDropdown(); onCreateTenant?.(); } }}>
              <TIcon name="add" className="tenant-create-icon" />
              <span className="tenant-create-label">{formatMessage(locale, 'tenant.create.action')}</span>
            </div>
          ) : null}
        </div>
        <div className="tenant-overlay" onClick={closeDropdown} />
      </> : null}
    </div>
  );
}
