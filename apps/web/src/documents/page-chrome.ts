// Pure helpers for the documents page chrome, ported from the Vue baseline
// frontend/src/views/knowledge/KnowledgeBase.vue (document branch). No React,
// no i18n — the components in DocumentsPageChrome.tsx consume these.

export interface ParserEngineLike {
  Name: string;
  FileTypes?: string[];
  Available?: boolean;
}

export interface ParserEngineRuleLike {
  file_types: string[];
  engine: string;
}

/**
 * Vue KnowledgeBase.vue supportedFileTypes (L186-216): a file type is
 * uploadable when its explicit chunking_config.parser_engine_rules entry
 * targets an available engine, or — absent an explicit rule — when the engine
 * declaring it is available.
 */
export function computeSupportedFileTypes(
  engines: readonly ParserEngineLike[],
  rules: readonly ParserEngineRuleLike[],
): Set<string> {
  if (engines.length === 0) return new Set<string>();
  const ruleMap = new Map<string, string>();
  for (const rule of rules) {
    for (const fileType of rule.file_types) ruleMap.set(fileType, rule.engine);
  }
  const availableEngineNames = new Set(
    engines.filter((engine) => engine.Available !== false).map((engine) => engine.Name),
  );
  const available = new Set<string>();
  for (const engine of engines) {
    for (const fileType of engine.FileTypes ?? []) {
      if (available.has(fileType)) continue;
      const explicitEngine = ruleMap.get(fileType);
      if (explicitEngine) {
        if (availableEngineNames.has(explicitEngine)) available.add(fileType);
      } else if (engine.Available !== false) {
        available.add(fileType);
      }
    }
  }
  return available;
}

/**
 * Vue KnowledgeBase.vue unsupportedFileTypes (L222-233): every type declared
 * by any engine minus the supported set, sorted — drives the parser-hint
 * warning line (部分文档类型（…）暂无可用解析引擎).
 */
export function computeUnsupportedFileTypes(
  engines: readonly ParserEngineLike[],
  rules: readonly ParserEngineRuleLike[],
): string[] {
  if (engines.length === 0) return [];
  const allTypes = new Set<string>();
  for (const engine of engines) {
    for (const fileType of engine.FileTypes ?? []) allTypes.add(fileType);
  }
  const supported = computeSupportedFileTypes(engines, rules);
  return [...allTypes].filter((fileType) => !supported.has(fileType)).sort();
}

/** Vue handleNavigateToKbList → router.push('/platform/knowledge-bases'). */
export const documentsKBListPath = '/platform/knowledge-bases';

/**
 * KBL-R1：KB 详情统一 platform 族路径（/platform/knowledge-bases/:id）。
 * 旧 library 族 /knowledgeBase/:id 已在 router 层重定向收编，内部 href
 * 一律直接生成 platform 族形态（对齐 Vue 全程 platform 族的基准）。
 */
export function documentsKBDetailPath(knowledgeBaseId: string): string {
  return '/platform/knowledge-bases/' + encodeURIComponent(knowledgeBaseId);
}

/**
 * KBW-2：KB 详情面包屑 tab 链接。Vue 的 tab 切换是就地
 * router.replace({ query: { tab } })（KnowledgeBase.vue L1150-1158 watcher），
 * URL 停留在当前路径上，回文档 tab 时 delete query.tab；React 用链接导航
 * 表达同一语义。KBL-R1 起 library 族入口已重定向收编，路径恒为 platform
 * 族（currentPathname 参数保留以兼容既有调用方签名）。documents tab 不带
 * tab 参数。
 */
export function kbTabHref(
  knowledgeBaseId: string,
  tab: 'documents' | 'wiki' | 'graph',
  _currentPathname: string,
): string {
  const base = documentsKBDetailPath(knowledgeBaseId);
  return tab === 'documents' ? base : `${base}?tab=${tab}`;
}

/**
 * Vue filterParams (L668-683): the date-range pair ("YYYY-MM-DD" inputs)
 * becomes inclusive backend bounds. The handler
 * (internal/handler/knowledge.go parseFilterTime) accepts both layouts.
 */
export function dateRangeToTimeParams(
  range: readonly (string | undefined)[] | undefined,
): { start_time?: string; end_time?: string } {
  const [start, end] = range ?? [];
  const params: { start_time?: string; end_time?: string } = {};
  if (start) params.start_time = `${start} 00:00:00`;
  if (end) params.end_time = `${end} 23:59:59`;
  return params;
}

/**
 * Vue folderTree.ts isFilteringDocuments: any active filter (beyond plain
 * folder browsing) widens the list scope to the folder's whole subtree.
 */
export function isFilteringDocuments(filters: {
  keyword?: string;
  tagIds?: string[];
  fileType?: string;
  parseStatus?: string;
  source?: string;
  timeRange?: readonly (string | undefined)[];
}): boolean {
  return !!(
    filters.keyword?.trim() ||
    filters.tagIds?.length ||
    filters.fileType ||
    filters.parseStatus ||
    filters.source ||
    filters.timeRange?.filter(Boolean).length
  );
}
