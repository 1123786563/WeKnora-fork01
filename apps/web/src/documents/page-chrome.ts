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

export function documentsKBDetailPath(knowledgeBaseId: string): string {
  return '/knowledgeBase/' + encodeURIComponent(knowledgeBaseId);
}

/** Vue gear (handleOpenKBSettings) lands on the React KB settings route. */
export function documentsKBSettingsPath(knowledgeBaseId: string): string {
  return '/knowledgeBase/' + encodeURIComponent(knowledgeBaseId) + '/settings';
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
