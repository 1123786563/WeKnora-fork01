import { useEffect, useMemo, useRef, useState } from "react";
import type {
  WikiPage as WikiPageModel,
  WikiPageRevision,
  WikiFolderNode,
  WikiIndexResponse,
  WeKnoraClient,
} from "@weknora/api-client";
import { diffWikiRevision } from "@weknora/domain/wiki/diff";
import { Button, Card, Dialog, Input, Status, Textarea } from "@weknora/ui";
import { DocumentsBreadcrumb, ParserHint, type DocumentsBreadcrumbTab, type KBChromeListItem } from "../documents/DocumentsPageChrome.tsx";
import { computeSupportedFileTypes, computeUnsupportedFileTypes, documentsKBSettingsPath } from "../documents/page-chrome.ts";
import { KnowledgeSettingsPage } from "../knowledge-settings/KnowledgeSettingsPage.tsx";
import { applyWikiSearch, overwriteWikiPage, saveWikiPage, validateWikiPageInput, wikiReaderEmptyState, wikiRevertCopy, type WikiSaveState } from "./editor.ts";
import { createSourceRefTitleHydrator, type SourceRefTitleHydrator } from "./source-titles.ts";
import {
  assembleWikiIndexMarkdown,
  handleWikiBodyClick,
  parseWikiSourceRefs,
  renderWikiMarkdown,
  stripDuplicateLeadingTitle,
  wikiSlugDisplayName,
} from "./markdown.ts";
import "./wiki-reader.css";
import { createTranslator, useAppLocale } from "../i18n.ts";
import { navigate } from "../platform/navigation.ts";
import { pagerState } from "../pagination.ts";
import { wikiEditPermission } from "./edit-permission.ts";
import { canUploadKnowledgeDocuments, resolveKBSurfaceTabs, type KBSurfaceKB, type KBSurfaceMe, type KBSurfaceTab } from "../knowledge/permissions.ts";

const WIKI_PAGE_SIZE = 50;

// Vue renders every sidebar icon as an inline t-icon SVG, so icon glyphs
// never appear in the accessible text. The React port mirrors that with a
// small stroke-icon set (aria-hidden, no text content).
export function WikiGlyph({ kind, size = 14 }: { kind: "search" | "index" | "tree" | "list" | "folder-add" | "page-add" | "chevron"; size?: number }) {
  const stroke = {
    stroke: "currentColor",
    strokeWidth: 1.8,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    fill: "none",
  };
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {kind === "search" ? <g {...stroke}><circle cx="11" cy="11" r="7" /><path d="M20 20l-4-4" /></g> : null}
      {kind === "index" ? <g {...stroke}><rect x="4" y="4" width="16" height="16" rx="2" /><path d="M9 4v16" /></g> : null}
      {kind === "tree" ? <g {...stroke}><path d="M4 6h16M4 12h10M4 18h13" /></g> : null}
      {kind === "list" ? <g {...stroke}><path d="M8 6h12M8 12h12M8 18h12M4 5.5h.01M4 11.5h.01M4 17.5h.01" /></g> : null}
      {kind === "folder-add" ? <g {...stroke}><path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2z" /><path d="M12 11v5M9.5 13.5h5" /></g> : null}
      {kind === "page-add" ? <g {...stroke}><path d="M6 3h8l4 4v14H6z" /><path d="M12 11v5M9.5 13.5h5" /></g> : null}
      {kind === "chevron" ? <g {...stroke}><path d="M9 6l6 6-6 6" /></g> : null}
    </svg>
  );
}

// Vue getPageIcon (WikiBrowser.vue:1980-1990) gives each page_type its own
// glyph in the sidebar tree; the React tree marks the type with the same
// color coding as the graph legend (KnowledgeGraphPage GRAPH_TYPE_DOT_BG).
const WIKI_TYPE_DOT: Record<string, string> = {
  summary: "bg-[#0052d9]",
  entity: "bg-[#2ba471]",
  concept: "bg-[#e37318]",
  synthesis: "bg-[#0594fa]",
  comparison: "bg-[#d54941]",
  index: "bg-[#8c8c8c]",
};

export function wikiRevertConfirmation(
  translate: (key: string, values?: Record<string, string | number>) => string,
  version: number,
): string {
  return wikiRevertCopy(translate, "confirm", version);
}

// ─── Vue picture-preview.vue (t-image-viewer) ───
//
// The Vue viewer opens with closeOnOverlay + closeOnEscKeydown and the
// TDesign default toolbar (zoom in/out, scale readout). Structure: dark
// mask, centered scalable image, top-right close.
const WIKI_PREVIEW_MIN_SCALE = 0.2;
const WIKI_PREVIEW_MAX_SCALE = 5;

export function wikiPreviewStep(scale: number, delta: number): number {
  return Math.min(WIKI_PREVIEW_MAX_SCALE, Math.max(WIKI_PREVIEW_MIN_SCALE, Math.round((scale + delta) * 100) / 100));
}

export function WikiImagePreview({ src, onClose }: { src: string; onClose: () => void }) {
  const [scale, setScale] = useState(1);
  useEffect(() => {
    // t-image-viewer closeOnEscKeydown
    const onKeydown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeydown);
    return () => window.removeEventListener("keydown", onKeydown);
  }, [onClose]);
  return (
    <div
      className="wk-wiki-img-preview fixed inset-0 z-[1100] flex items-center justify-center"
      role="dialog"
      aria-modal="true"
      onClick={(event) => {
        // t-image-viewer closeOnOverlay: only a mask click closes, not
        // clicks on the image or the toolbar.
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="wk-wiki-img-preview-mask absolute inset-0 bg-black/85" aria-hidden="true" />
      <img
        className="wk-wiki-img-preview-image relative max-h-[90vh] max-w-[90vw] select-none object-contain"
        src={src}
        alt=""
        style={{ transform: `scale(${scale})` }}
        draggable={false}
      />
      <div className="wk-wiki-img-preview-toolbar absolute bottom-6 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-md bg-black/60 px-3 py-2 text-white">
        <button
          type="button"
          className="wk-wiki-img-preview-zoom-out cursor-pointer border-0 bg-transparent px-2 py-1 text-lg leading-none text-white"
          onClick={() => setScale((value) => wikiPreviewStep(value, -0.25))}
        >
          −
        </button>
        <span className="wk-wiki-img-preview-scale min-w-[3.5rem] text-center text-xs tabular-nums">
          {Math.round(scale * 100)}%
        </span>
        <button
          type="button"
          className="wk-wiki-img-preview-zoom-in cursor-pointer border-0 bg-transparent px-2 py-1 text-lg leading-none text-white"
          onClick={() => setScale((value) => wikiPreviewStep(value, 0.25))}
        >
          +
        </button>
      </div>
      <button
        type="button"
        className="wk-wiki-img-preview-close absolute right-6 top-6 flex h-9 w-9 cursor-pointer items-center justify-center rounded-full border-0 bg-black/60 text-xl leading-none text-white"
        aria-label="×"
        onClick={onClose}
      >
        ×
      </button>
    </div>
  );
}

// ─── Vue reader footer: backlinks + sources ───
//
// WikiBrowser.vue lines 591-612: a `Linked from` row built from the page's
// `in_links` and a `Source documents` row built from `parsedSourceRefs`.
// `in_links` / `source_refs` ride through the api-client page response's
// index signature (backend WikiPage JSON includes both).

export function WikiReaderFooter({
  page,
  resolveSlugName,
  translate,
  onNavigate,
  onOpenSourceDoc,
  sourceTitles,
}: {
  page: { [key: string]: unknown; in_links?: unknown; source_refs?: unknown };
  resolveSlugName: (slug: string) => string;
  translate?: (key: string) => string;
  onNavigate: (slug: string) => void;
  onOpenSourceDoc?: (documentId: string) => void;
  /** Hydrated id → title map from the Vue sourceRefTitleCache port. */
  sourceTitles?: Record<string, string>;
}) {
  const t = translate ?? ((key: string) => key);
  const inLinks = Array.isArray(page.in_links)
    ? page.in_links.filter((slug): slug is string => typeof slug === "string" && slug.length > 0)
    : [];
  const sources = parseWikiSourceRefs(page.source_refs);
  if (inLinks.length === 0 && sources.length === 0) return null;
  return (
    <footer className="wiki-reader-footer">
      {inLinks.length > 0 ? (
        <div className="wiki-reader-footer-row">
          <span className="wiki-reader-footer-label">{t("wikiBrowser.linkedFrom")}</span>
          <span className="wiki-reader-footer-value">
            {inLinks.map((link) => (
              <a
                key={`in-${link}`}
                href="#"
                className="wiki-content-link"
                data-slug={link}
                onClick={(event) => {
                  event.preventDefault();
                  onNavigate(link);
                }}
              >
                {resolveSlugName(link)}
              </a>
            ))}
          </span>
        </div>
      ) : null}
      {sources.length > 0 ? (
        <div className="wiki-reader-footer-row">
          <span className="wiki-reader-footer-label">{t("wikiBrowser.sources")}</span>
          <span className="wiki-reader-footer-value">
            {sources.map((ref) => (
              <a
                key={ref.id}
                href="#"
                className="wiki-content-link"
                data-source-id={ref.id}
                onClick={(event) => {
                  event.preventDefault();
                  onOpenSourceDoc?.(ref.id);
                }}
              >
                {sourceTitles?.[ref.id] ?? ref.title}
              </a>
            ))}
          </span>
        </div>
      ) : null}
    </footer>
  );
}

// ─── Vue index system view ───
//
// The index overview renders as markdown through the same pipeline as a
// regular page body (intro + `## Label (total)` directory sections), so
// [[wiki-link]] and image clicks behave identically (WikiBrowser.vue
// `renderedIndexMarkdown`).

const WIKI_INDEX_TYPE_LABEL_KEYS: Record<string, string> = {
  knowledge: "wikiBrowser.filterKnowledge",
  summary: "wikiBrowser.filterSummary",
  entity: "wikiBrowser.filterEntity",
  concept: "wikiBrowser.filterConcept",
  synthesis: "wikiBrowser.filterSynthesis",
  comparison: "wikiBrowser.filterComparison",
};

export function WikiIndexView({
  indexView,
  loading,
  error,
  hasMore,
  labelFor,
  onLoadMore,
  onNavigate,
  onOpenImage,
  locale,
}: {
  indexView: WikiIndexResponse;
  loading: boolean;
  error: string | null;
  hasMore: boolean;
  labelFor: (type: string) => string;
  onLoadMore: () => void;
  onNavigate: (slug: string) => void;
  onOpenImage: (src: string) => void;
  locale?: ReturnType<typeof useAppLocale>;
}) {
  const fallbackLocale = useAppLocale();
  const t = createTranslator(locale ?? fallbackLocale);
  const markdown = assembleWikiIndexMarkdown({
    intro: indexView.intro,
    groups: indexView.groups,
    labelFor,
  });
  if (error) return <Status tone="error">{error}</Status>;
  if (loading && !markdown) {
    return <Status>{t("wikiBrowser.loading")}</Status>;
  }
  if (!markdown) {
    return (
      <div className="wiki-reader-empty">
        <p className="wiki-empty-title">{t("wikiBrowser.indexEmpty")}</p>
      </div>
    );
  }
  return (
    <>
      <div
        className="wiki-reader-body wiki-index-body m-0 box-border min-h-[22rem]"
        onClick={(event) => handleWikiBodyClick(event, { navigate: onNavigate, openImage: onOpenImage })}
        dangerouslySetInnerHTML={{
          __html: renderWikiMarkdown(markdown, { resolveSlugName: (slug) => slug }),
        }}
      />
      {hasMore ? (
        <div className="wiki-index-sentinel flex items-center justify-center px-0 pb-6 pt-4">
          <Button type="button" disabled={loading} onClick={onLoadMore}>
            {loading ? t("wikiBrowser.loading") : t("wikiBrowser.loadMoreShort")}
          </Button>
        </div>
      ) : null}
    </>
  );
}

export function WikiPage({
  client,
  knowledgeBaseId,
  initialSlug,
  canContribute: canContributeProp = false,
  onOpenSourceDoc,
}: {
  client: WeKnoraClient;
  knowledgeBaseId: string;
  initialSlug?: string;
  canContribute?: boolean;
  /** Vue `open-source-doc` emit: open a wiki source document in the knowledge surface. */
  onOpenSourceDoc?: (documentId: string) => void;
}) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [page, setPage] = useState(1);
  const [searchDraft, setSearchDraft] = useState("");
  const [keyword, setKeyword] = useState("");
  const [pages, setPages] = useState<WikiPageModel[]>([]);
  // Vue WikiBrowser sidebar buckets: 知识 folds entity/concept/synthesis/
  // comparison; 摘要 keeps its own tab. Counts come from /wiki/stats.
  const [pagesByType, setPagesByType] = useState<Record<string, number>>({});
  const [activeBucket, setActiveBucket] = useState("");
  const [pageTotal, setPageTotal] = useState(0);
  const [viewMode, setViewMode] = useState<"tree" | "list">("tree");
  const [folderId, setFolderId] = useState("");
  const [folderPath, setFolderPath] = useState("");
  const [folderTrail, setFolderTrail] = useState<Array<{ id: string; name: string; path: string }>>([]);
  const [folders, setFolders] = useState<WikiFolderNode[]>([]);
  // Vue expandable directory tree (WikiBrowser.vue toggleDirectory): folders
  // expand in place; children (sub-folders + pages) load lazily per path.
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
  const [dirChildren, setDirChildren] = useState<Record<string, { folders: WikiFolderNode[]; pages: WikiPageModel[] }>>({});
  const [dirLoading, setDirLoading] = useState<Record<string, boolean>>({});
  const [indexView, setIndexView] = useState<WikiIndexResponse | null>(null);
  const [indexLoading, setIndexLoading] = useState(false);
  const [indexError, setIndexError] = useState<string | null>(null);
  const [indexNextCursor, setIndexNextCursor] = useState<string | null>(null);
  const [folderBusy, setFolderBusy] = useState(false);
  const [selected, setSelected] = useState<WikiPageModel | null>(null);
  const [title, setTitle] = useState("");
  const [slug, setSlug] = useState("");
  const [summary, setSummary] = useState("");
  const [content, setContent] = useState("");
  const [editing, setEditing] = useState(false);
  const [version, setVersion] = useState(1);
  const [state, setState] = useState<{
    status: "loading" | "error" | "success";
    message?: string;
  }>({ status: "loading" });
  const [saveState, setSaveState] = useState<WikiSaveState | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [revisions, setRevisions] = useState<WikiPageRevision[]>([]);
  const [revision, setRevision] = useState<WikiPageRevision | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [reverting, setReverting] = useState(false);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [canContribute, setCanContribute] = useState(canContributeProp);
  // Shared documents-page chrome inputs (KnowledgeGraphPage parity): the KB
  // switcher list, parser engines for the info card / parser hint, and the
  // in-place KB settings dialog behind the ⚙ button.
  const [kbMeta, setKbMeta] = useState<KBSurfaceKB | null>(null);
  const [kbList, setKbList] = useState<KBChromeListItem[]>([]);
  const [canManage, setCanManage] = useState(false);
  const [parserEngines, setParserEngines] = useState<{ Name: string; FileTypes?: string[]; Available?: boolean }[]>([]);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const supportedFileTypes = useMemo(() => {
    const rules = (kbMeta?.chunking_config as { parser_engine_rules?: { file_types: string[]; engine: string }[] } | null | undefined)?.parser_engine_rules ?? [];
    return [...computeSupportedFileTypes(parserEngines, rules)];
  }, [kbMeta, parserEngines]);
  const unsupportedFileTypes = useMemo(
    () => computeUnsupportedFileTypes(parserEngines, (kbMeta?.chunking_config as { parser_engine_rules?: { file_types: string[]; engine: string }[] } | null | undefined)?.parser_engine_rules ?? []),
    [kbMeta, parserEngines],
  );
  // Vue title row (KnowledgeBase.vue L2359-2380): the third crumb level is the
  // 文档 / Wiki / 图谱 row with the active Wiki tab.
  const kbBasePath = `/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}`;
  const resolvedTabs = kbMeta ? resolveKBSurfaceTabs(kbMeta) : undefined;
  const kbTabs: DocumentsBreadcrumbTab[] | undefined = resolvedTabs
    ? resolvedTabs.map((tab: KBSurfaceTab) => ({
      key: tab,
      label: tab === "documents"
        ? t("knowledgeEditor.wikiBrowser.tabDocuments")
        : tab === "wiki"
          ? "Wiki"
          : t("knowledgeEditor.wikiBrowser.tabGraph"),
      href: tab === "documents" ? kbBasePath : `${kbBasePath}?tab=${tab}`,
      active: tab === "wiki",
      title: tab === "graph" ? t("knowledgeEditor.wikiBrowser.tabGraphTip") : undefined,
    }))
    : undefined;
  // Vue picture-preview state: the previewed image src, null closes the viewer.
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  // Vue sourceRefTitleCache: hydrated source-document titles keyed by bare
  // knowledge id, resolved through the shared documents detail endpoint
  // (GET /api/v1/knowledge/{id}) — same capability the Vue browser uses.
  const [sourceTitles, setSourceTitles] = useState<Record<string, string>>({});
  const sourceTitleHydrator = useRef<{ client: WeKnoraClient; hydrator: SourceRefTitleHydrator } | null>(null);
  if (!sourceTitleHydrator.current || sourceTitleHydrator.current.client !== client) {
    sourceTitleHydrator.current = {
      client,
      hydrator: createSourceRefTitleHydrator((id) => client.knowledge.documents.get(id)),
    };
  }
  const rawSourceRefs = (selected as { source_refs?: unknown } | null)?.source_refs;
  const selectedSourceRefs = Array.isArray(rawSourceRefs)
    ? rawSourceRefs.filter((ref): ref is string => typeof ref === "string" && ref.length > 0)
    : [];
  const selectedSourceRefsKey = selectedSourceRefs.join("\u0000");
  useEffect(() => {
    if (selectedSourceRefsKey.length === 0) return;
    let active = true;
    void sourceTitleHydrator.current!.hydrator.hydrate(selectedSourceRefsKey.split("\u0000")).then((titles) => {
      if (active) setSourceTitles(titles);
    });
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedSourceRefsKey]);

  useEffect(() => {
    let active = true;
    setCanContribute(canContributeProp);
    // The wiki surface shares the documents-page chrome (breadcrumb switcher,
    // ⓘ file-type card, ⚙ settings dialog), so it sources the same inputs as
    // KnowledgeGraphPage: kb settings, me, the tenant KB list, parser engines.
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.auth.me().catch(() => null),
      // Vue canEdit consults the org shared-knowledge-bases list as the
      // authoritative share-grant signal; the probe mirrors those inputs.
      client.identity.organizations.knowledgeBaseShares.listShared().catch(() => null),
      client.knowledgeBases.list().catch(() => []),
      client.knowledgeBases.settings.parserEngines().catch(() => ({ data: [] })),
    ] as const).then(([kb, me, sharedRows, list, engines]) => {
      if (!active) return;
      setKbMeta(kb as KBSurfaceKB);
      setCanManage(canUploadKnowledgeDocuments(kb as KBSurfaceKB, me as KBSurfaceMe | null));
      setKbList((list as { id: unknown; name: unknown }[]).map((item) => ({ id: String(item.id), name: String(item.name) })));
      setParserEngines((engines.data ?? []) as { Name: string; FileTypes?: string[]; Available?: boolean }[]);
      if (!me) return;
      setCanContribute(
        wikiEditPermission(kb as KBSurfaceKB, me as KBSurfaceMe, sharedRows),
      );
    }).catch(() => {
      // Keep the caller's role gate when the optional permission probe fails.
    });
    return () => { active = false; };
  }, [client, knowledgeBaseId, canContributeProp]);

  // Vue space-epoch parity: a slow unfiltered response must not clobber the
  // newer bucket-filtered one.
  const loadEpoch = useRef(0);
  // The index overview auto-open runs once per mount, not on every reload.
  const didAutoOpenIndex = useRef(false);
  async function loadPages() {
    const run = ++loadEpoch.current;
    setState({ status: "loading" });
    try {
      // Must-fix #3: server-backed pagination replaces the 50-entry hard cap.
      const response = await client.wiki.list(knowledgeBaseId, {
        page,
        page_size: WIKI_PAGE_SIZE,
        keyword: keyword || undefined,
        ...(viewMode === "tree" && folderPath ? { category_path: folderPath } : {}),
        // The backend pages list ignores page_types — Vue buckets client-side.
      });
      if (run !== loadEpoch.current) return; // stale response: drop
      // The full (unfiltered) list feeds every sidebar bucket count; the
      // active bucket narrows the visible rows at render time — Vue keeps
      // per-type buckets independent of the active tab.
      setPages(response.pages);
      setPageTotal(response.total ?? response.pages.length);
      const requested = initialSlug?.trim();
      const requestedPage = requested
        ? response.pages.find((page) => page.slug === requested)
        : undefined;
      if (requestedPage) {
        choose(requestedPage);
      } else if (!requested && !didAutoOpenIndex.current && response.pages.length > 0) {
        // Vue WikiBrowser loadPages: without a ?slug= deep link the index
        // overview is the natural landing view (openIndexView).
        didAutoOpenIndex.current = true;
        void openIndex();
      }
      setState({ status: "success" });
    } catch (error) {
      setState({
        status: "error",
        message:
          error instanceof Error
            ? error.message
            : t("wikiBrowser.revisionLoadFailed"),
      });
    }
  }
  useEffect(() => {
    void loadPages();
  }, [client, knowledgeBaseId, keyword, initialSlug, page, folderPath, viewMode, activeBucket]);
  useEffect(() => {
    let active = true;
    void client.wiki.stats(knowledgeBaseId).then((value) => {
      if (active) {
        setPagesByType(value.pages_by_type);
        // Vue preferredDefaultTab: 知识 first, then 摘要 — applied only while
        // the user has not picked a bucket themselves.
        setActiveBucket((current) => {
          if (current) return current;
          const knowledge = (value.pages_by_type.entity ?? 0) + (value.pages_by_type.concept ?? 0)
            + (value.pages_by_type.synthesis ?? 0) + (value.pages_by_type.comparison ?? 0);
          if (knowledge > 0) return "knowledge";
          if ((value.pages_by_type.summary ?? 0) > 0) return "summary";
          return "";
        });
      }
    }).catch(() => { if (active) setPagesByType({}); });
    return () => { active = false; };
  }, [client, knowledgeBaseId]);
  useEffect(() => {
    setPage(1);
  }, [keyword, folderPath, viewMode]);

  useEffect(() => {
    if (viewMode !== "tree") return;
    if (typeof client.wiki.folders !== "function") return;
    // Same bucket page_types the reload path passes (Vue loadCategoriesForType).
    const pageTypes = activeBucket === "knowledge"
      ? ["entity", "concept", "synthesis", "comparison"]
      : activeBucket === "summary" ? ["summary"] : [];
    void client.wiki.folders(knowledgeBaseId, folderId, pageTypes).then((result) => setFolders(result.folders)).catch(() => setFolders([]));
  }, [client, knowledgeBaseId, folderId, viewMode, activeBucket]);

  async function reloadFolders() {
    // Vue passes the active bucket's page_types so empty folders surface only
    // in the merged knowledge view (ListChildFolders showEmptyFolders rule).
    const pageTypes = activeBucket === "knowledge"
      ? ["entity", "concept", "synthesis", "comparison"]
      : activeBucket === "summary" ? ["summary"] : [];
    const result = await client.wiki.folders(knowledgeBaseId, folderId, pageTypes);
    setFolders(result.folders);
  }

  // Vue toggleDirectory (WikiBrowser.vue:1706): expanding a directory lazily
  // loads its child folders and its pages; collapsing just hides the rows.
  async function ensureDirChildren(folder: WikiFolderNode) {
    const key = folder.path;
    if (dirChildren[key] || dirLoading[key]) return;
    setDirLoading((current) => ({ ...current, [key]: true }));
    const pageTypes = activeBucket === "knowledge"
      ? ["entity", "concept", "synthesis", "comparison"]
      : activeBucket === "summary" ? ["summary"] : [];
    try {
      const [childFolders, childPages] = await Promise.all([
        client.wiki.folders(knowledgeBaseId, folder.id, pageTypes).catch(() => ({ folders: [] })),
        client.wiki.list(knowledgeBaseId, { page: 1, page_size: WIKI_PAGE_SIZE, category_path: folder.path }).catch(() => ({ pages: [] as WikiPageModel[] })),
      ]);
      setDirChildren((current) => ({ ...current, [key]: { folders: childFolders.folders, pages: childPages.pages } }));
    } finally {
      setDirLoading((current) => ({ ...current, [key]: false }));
    }
  }

  function toggleDirectory(folder: WikiFolderNode) {
    const key = folder.path;
    const willExpand = !expandedDirs.has(key);
    setExpandedDirs((current) => {
      const next = new Set(current);
      if (willExpand) next.add(key); else next.delete(key);
      return next;
    });
    if (willExpand) void ensureDirChildren(folder);
  }


  async function openIndex() {
    if (typeof client.wiki.index !== "function") return;
    // Vue openIndexView: entering the index overview clears the selected page
    // (and any open editor) — the reader area shows one thing at a time.
    setSelected(null);
    setEditing(false);
    setIndexLoading(true);
    setIndexError(null);
    try {
      const result = await client.wiki.index(knowledgeBaseId, { limit: WIKI_PAGE_SIZE });
      setIndexView(result);
      setIndexNextCursor(result.groups.map((group) => group.next_cursor).find(Boolean) ?? null);
    }
    catch (error) { setIndexError(error instanceof Error ? error.message : t("wikiBrowser.revisionLoadFailed")); setIndexView({ intro: "", version: 0, groups: [] }); setIndexNextCursor(null); }
    finally { setIndexLoading(false); }
  }

  async function loadMoreIndex() {
    if (!indexNextCursor || indexLoading || typeof client.wiki.index !== "function") return;
    setIndexLoading(true);
    setIndexError(null);
    try {
      const result = await client.wiki.index(knowledgeBaseId, { limit: WIKI_PAGE_SIZE, cursor: indexNextCursor });
      setIndexView((current) => current ? { ...current, groups: current.groups.map((group) => { const incoming = result.groups.find((item) => item.type === group.type); return incoming ? { ...group, total: incoming.total, items: [...group.items, ...incoming.items], next_cursor: incoming.next_cursor } : group; }) } : result);
      setIndexNextCursor(result.groups.map((group) => group.next_cursor).find(Boolean) ?? null);
    } catch (error) { setIndexError(error instanceof Error ? error.message : t("wikiBrowser.revisionLoadFailed")); }
    finally { setIndexLoading(false); }
  }

  function openFolder(folder: WikiFolderNode) {
    setFolderId(folder.id);
    setFolderPath(folder.path);
    setFolderTrail((current) => [...current, { id: folder.id, name: folder.name, path: folder.path }]);
    setPage(1);
    setIndexView(null);
  }

  function backFolder() {
    const previous = folderTrail.slice(0, -1);
    const parent = previous[previous.length - 1];
    setFolderTrail(previous);
    setFolderId(parent?.id ?? "");
    setFolderPath(parent?.path ?? "");
    setPage(1);
  }

  function switchViewMode(next: "tree" | "list") {
    setViewMode(next);
    setIndexView(null);
    setFolderId("");
    setFolderPath("");
    setFolderTrail([]);
    setPage(1);
  }

  function submitSearch() {
    const next = applyWikiSearch(searchDraft);
    setSearchDraft(next.draft);
    setKeyword(next.keyword);
    setPage(1);
  }

  // Vue inline editing (WikiFolderActions / wiki-directory-item--editing):
  // creation and rename happen in an input embedded in the row, not a prompt.
  const [inlineCreating, setInlineCreating] = useState(false);
  const [inlineCreatingName, setInlineCreatingName] = useState("");
  const [renamingFolderId, setRenamingFolderId] = useState("");
  const [renamingName, setRenamingName] = useState("");

  async function createFolder(name?: string) {
    const finalName = (name ?? window.prompt(t("wikiBrowser.folderNamePlaceholder")) ?? "").trim();
    if (!finalName || folderBusy) return;
    setFolderBusy(true);
    try { await client.wiki.createFolder(knowledgeBaseId, folderId, finalName); await reloadFolders(); }
    catch (error) { setState({ status: "error", message: error instanceof Error ? error.message : t("wikiBrowser.createFolderFailed") }); }
    finally { setFolderBusy(false); }
  }

  function startInlineCreate() {
    setInlineCreating(true);
    setInlineCreatingName("");
  }

  async function submitInlineCreate() {
    const name = inlineCreatingName.trim();
    setInlineCreating(false);
    setInlineCreatingName("");
    if (name) await createFolder(name);
  }

  function startRenameFolder(folder: WikiFolderNode) {
    setRenamingFolderId(folder.id);
    setRenamingName(folder.name);
  }

  async function renameFolder(folder: WikiFolderNode, name?: string) {
    const finalName = (name ?? window.prompt(t("wikiBrowser.folderNamePlaceholder"), folder.name) ?? "").trim();
    if (!finalName || finalName === folder.name || folderBusy) return;
    setFolderBusy(true);
    try { await client.wiki.updateFolder(knowledgeBaseId, folder.id, { name: finalName }); await reloadFolders(); }
    catch (error) { setState({ status: "error", message: error instanceof Error ? error.message : t("wikiBrowser.renameFolderFailed") }); }
    finally { setFolderBusy(false); }
  }

  function movePageToFolder(slug: string, folder: WikiFolderNode) {
    if (folderBusy) return;
    setFolderBusy(true);
    void client.wiki.movePage(knowledgeBaseId, slug, folder.id)
      .then(() => { void loadPages(); void reloadFolders(); })
      .catch((reason: unknown) => setState({ status: "error", message: reason instanceof Error ? reason.message : t("wikiBrowser.movePageFailed") }))
      .finally(() => setFolderBusy(false));
  }

  async function deleteFolder(folder: WikiFolderNode) {
    if (!window.confirm(t("wikiBrowser.deleteFolderConfirm", { name: folder.name })) || folderBusy) return;
    setFolderBusy(true);
    try { await client.wiki.removeFolder(knowledgeBaseId, folder.id); await reloadFolders(); }
    catch (error) { setState({ status: "error", message: error instanceof Error ? error.message : t("wikiBrowser.deleteFolderFailed") }); }
    finally { setFolderBusy(false); }
  }

  function choose(page: WikiPageModel) {
    setSelected(page);
    // Vue navigateToSlug clears the index overview when a page is opened.
    setIndexView(null);
    setTitle(page.title);
    setSlug(page.slug);
    setSummary(page.summary);
    setContent(page.content);
    setVersion(page.version);
    setSaveState(null);
    setEditing(false);
  }
  function newPage() {
    if (!canContribute) return;
    setSelected(null);
    setTitle("");
    setSlug("");
    setSummary("");
    setContent("");
    setVersion(1);
    setSaveState(null);
    setEditing(true);
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canContribute) return;
    if (!selected) {
      const validationError = validateWikiPageInput(
        { title, content },
        { titleRequired: t("wikiBrowser.newPageMissingFields"), contentRequired: t("wikiBrowser.newPageMissingFields"), conflict: t("wikiBrowser.editSaveFailed"), saveFailed: t("wikiBrowser.newPageFailed") },
      );
      if (validationError) {
        setSaveState({ status: "error", message: validationError });
        return;
      }
      try {
        const page = await client.wiki.create(knowledgeBaseId, {
          title,
          slug,
          summary,
          content,
          version: 1,
        });
        choose(page);
        setSaveState({ status: "saved", page });
        await loadPages();
      } catch (error) {
        setSaveState({
          status: "error",
          message:
            error instanceof Error
              ? error.message
              : t("wikiBrowser.newPageFailed"),
        });
      }
      return;
    }
    const result = await saveWikiPage(
      client.wiki,
      knowledgeBaseId,
      selected.slug,
      { title, content, summary, version },
      {
        titleRequired: t("wikiBrowser.newPageMissingFields"),
        contentRequired: t("wikiBrowser.newPageMissingFields"),
        conflict: t("wikiBrowser.editSaveFailed"),
        saveFailed: t("wikiBrowser.editSaveFailed"),
      },
    );
    setSaveState(result);
    if (result.status === "saved") {
      choose(result.page);
      await loadPages();
    }
  }

  async function reloadSelected() {
    if (!selected) return;
    try {
      const latest = await client.wiki.get(knowledgeBaseId, selected.slug);
      // Vue `reloadLatestIntoEditor`: discard the local draft and re-open the
      // editor seeded with the server's current content, staying in edit mode.
      choose(latest);
      setEditing(true);
    } catch (error) {
      setSaveState({
        status: "error",
        message:
          error instanceof Error
            ? error.message
            : t("wikiBrowser.editSaveFailed"),
      });
    }
  }

  // Vue `cancelEditPage`: exit edit mode and drop the in-progress draft.
  function cancelEdit() {
    if (!selected) return;
    choose(selected);
  }

  // Vue WikiBrowser.vue `navigateToSlug`: follow a [[wiki-link]] by loading
  // the target page into the reader. A missing slug keeps the current page
  // (the error is only logged), matching the Vue behavior.
  function navigateToSlug(nextSlug: string) {
    client.wiki
      .get(knowledgeBaseId, nextSlug)
      .then(choose)
      .catch((error: unknown) => {
        console.error(`Failed to navigate to ${nextSlug}:`, error);
      });
  }

  // Vue `overwriteSavePage`: resolve a 409 conflict by re-saving the local
  // draft on top of the server's latest version (last write wins; the losing
  // version stays in revision history).
  async function overwriteConflict() {
    if (!selected) return;
    const result = await overwriteWikiPage(
      client.wiki,
      knowledgeBaseId,
      selected.slug,
      { title, content, summary },
      {
        titleRequired: t("wikiBrowser.newPageMissingFields"),
        contentRequired: t("wikiBrowser.newPageMissingFields"),
        conflict: t("wikiBrowser.editSaveFailed"),
        saveFailed: t("wikiBrowser.editSaveFailed"),
      },
    );
    setSaveState(result);
    if (result.status === "saved") {
      choose(result.page);
      await loadPages();
    }
  }

  async function deleteWikiPage() {
    if (!canContribute || !selected || deleteBusy) return;
    if (!window.confirm(t("wikiBrowser.deletePageConfirm", { title: selected.title }))) return;
    setDeleteBusy(true);
    try {
      await client.wiki.remove(knowledgeBaseId, selected.slug);
      setSelected(null);
      setEditing(false);
      setSaveState(null);
      await loadPages();
    } catch (error) {
      setSaveState({
        status: "error",
        message: error instanceof Error ? error.message : t("wikiBrowser.deletePageFailed"),
      });
    } finally {
      setDeleteBusy(false);
    }
  }

  async function openHistory() {
    if (!selected) return;
    setHistoryOpen(true);
    setHistoryLoading(true);
    setHistoryError(null);
    setRevision(null);
    try {
      setRevisions(
        (
          await client.wiki.revisions(knowledgeBaseId, selected.slug, {
            limit: 50,
            offset: 0,
          })
        ).revisions,
      );
    } catch (error) {
      setHistoryError(
        error instanceof Error
          ? error.message
          : t("wikiBrowser.revisionLoadFailed"),
      );
    } finally {
      setHistoryLoading(false);
    }
  }
  async function chooseRevision(item: WikiPageRevision) {
    setRevision(item);
    setHistoryLoading(true);
    setHistoryError(null);
    try {
      setRevision(
        await client.wiki.getRevision(knowledgeBaseId, item.slug, item.version),
      );
    } catch (error) {
      setHistoryError(
        error instanceof Error
          ? error.message
          : t("wikiBrowser.revisionLoadFailed"),
      );
    } finally {
      setHistoryLoading(false);
    }
  }
  async function revertRevision() {
    if (
      !canContribute ||
      !selected ||
      !revision ||
      !window.confirm(
        wikiRevertConfirmation(t, revision.version),
      )
    )
      return;
    setReverting(true);
    setHistoryError(null);
    try {
      const page = await client.wiki.revert(
        knowledgeBaseId,
        selected.slug,
        revision.version,
      );
      choose(page);
      setHistoryOpen(false);
      setSaveState({ status: "saved", page });
      await loadPages();
    } catch (error) {
      setHistoryError(
        error instanceof Error
          ? error.message
          : wikiRevertCopy(t, "failed", revision.version),
      );
    } finally {
      setReverting(false);
    }
  }
  // Vue WikiBrowser keeps the auto-generated index page out of the page list
  // (it lives in the 索引 view) and shows the 暂无 Wiki 页面 empty state when
  // only index pages exist.
  const listPages = pages.filter((page) => String((page as Record<string, unknown>).page_type ?? "") !== "index");
  const pager = pagerState(pageTotal, page, WIKI_PAGE_SIZE);
  const revisionDiff = useMemo(
    () =>
      revision && selected
        ? diffWikiRevision(
            {
              title: revision.title,
              summary: revision.summary,
              content: revision.content ?? "",
            },
            {
              title: selected.title,
              summary: selected.summary,
              content: selected.content,
            },
          )
        : [],
    [revision, selected],
  );
  const KNOWLEDGE_TYPES = ["entity", "concept", "synthesis", "comparison"];
  // Vue's sidebar tab count reflects the pages visible at the current tree
  // level (WikiBrowser visibleTabs total comes from the per-level directory
  // state), so in tree mode only root-level pages count toward the tab.
  const isRootPage = (page: WikiPageModel) => {
    const cp = (page as Record<string, unknown>).category_path as string[] | undefined;
    return !cp || cp.length === 0;
  };
  const pageInBucket = (page: WikiPageModel, bucket: string) => {
    const types = bucket === "knowledge" ? KNOWLEDGE_TYPES : bucket === "summary" ? ["summary"] : null;
    return !types || types.includes(String((page as Record<string, unknown>).page_type ?? ""));
  };
  const rootPages = listPages.filter(isRootPage);
  const bucketRootPages = rootPages.filter((page) => pageInBucket(page, activeBucket));
  const rootCountFor = (types: readonly string[]) => rootPages.filter((page) => types.includes(String((page as Record<string, unknown>).page_type ?? ""))).length;
  const bucketTabs = (["knowledge", "summary"] as const)
    .map((type) => {
      const types = type === "knowledge" ? KNOWLEDGE_TYPES : [type];
      const statTotal = type === "knowledge"
        ? KNOWLEDGE_TYPES.reduce((sum, key) => sum + (pagesByType[key] ?? 0), 0)
        : (pagesByType[type] ?? 0);
      return {
        type,
        label: type === "knowledge" ? t("wikiBrowser.filterKnowledge") : t("wikiBrowser.filterSummary"),
        total: viewMode === "tree" ? rootCountFor(types) : statTotal,
      };
    })
    .filter((tab) => tab.total > 0);
  // Vue activeTreeRows: an interleaved directory/page row list for the tree
  // view — expanded folders show their child folders and pages inline.
  const treeRows: Array<{ kind: "dir"; folder: WikiFolderNode; depth: number } | { kind: "page"; page: WikiPageModel; depth: number }> = [];
  if (viewMode === "tree") {
    const walk = (fs: WikiFolderNode[], depth: number) => {
      for (const folder of fs) {
        treeRows.push({ kind: "dir", folder, depth });
        if (expandedDirs.has(folder.path)) {
          const children = dirChildren[folder.path];
          if (children) {
            walk(children.folders, depth + 1);
            for (const page of children.pages.filter((entry) => String((entry as Record<string, unknown>).page_type ?? "") !== "index")) {
              treeRows.push({ kind: "page", page, depth: depth + 1 });
            }
          }
        }
      }
    };
    walk(folders, 0);
    for (const page of bucketRootPages) treeRows.push({ kind: "page", page, depth: 0 });
  }
  const viewToggle = (
    <div className="wiki-view-toggle inline-flex items-center overflow-hidden rounded-[6px] border border-[#e4e7ec]" role="group" aria-label={t("wikiBrowser.viewModeToggle")}>
      <button type="button" className={`h-[26px] cursor-pointer border-0 bg-transparent px-[7px] [font:inherit] ${viewMode === "tree" ? "bg-[#eef4ef] text-[#07c05f]" : "bg-transparent text-[#66758b]"}`} aria-pressed={viewMode === "tree"} aria-label={t("wikiBrowser.viewTree")} title={t("wikiBrowser.viewTree")} onClick={() => switchViewMode("tree")}><WikiGlyph kind="tree" /></button>
      <button type="button" className={`h-[26px] cursor-pointer border-0 border-l border-l-[#e4e7ec] bg-transparent px-[7px] [font:inherit] ${viewMode === "list" ? "bg-[#eef4ef] text-[#07c05f]" : "bg-transparent text-[#66758b]"}`} aria-pressed={viewMode === "list"} aria-label={t("wikiBrowser.viewList")} title={t("wikiBrowser.viewList")} onClick={() => switchViewMode("list")}><WikiGlyph kind="list" /></button>
    </div>
  );
  const tabActions = (
    <div className="wiki-tab-bar-actions flex shrink-0 items-center gap-[2px]">
      {viewToggle}
      {canContribute ? <Button type="button" className="wiki-tab-bar-action" disabled={folderBusy} aria-label={t("wikiBrowser.newRootFolder")} title={t("wikiBrowser.newRootFolder")} onClick={startInlineCreate}><WikiGlyph kind="folder-add" /></Button> : null}
      {canContribute ? <Button type="button" className="wiki-tab-bar-action" aria-label={t("wikiBrowser.newPageBtn")} title={t("wikiBrowser.newPageBtn")} onClick={newPage}><WikiGlyph kind="page-add" /></Button> : null}
    </div>
  );
  const directory = (
    <>
      <div className="wiki-tab-bar flex items-center gap-1 pb-2" role="tablist" aria-label={t("wikiBrowser.viewModeToggle")}>
        <div className="wiki-tab-bar-scroll flex min-w-0 flex-1 items-center gap-1">
          {bucketTabs.map((tab) => <button
            key={tab.type}
            type="button"
            role="tab"
            aria-selected={activeBucket === tab.type}
            className={`wiki-tab flex cursor-pointer items-baseline gap-[3px] border-0 border-b-2 border-b-transparent bg-transparent px-[0.55rem] py-[0.45rem] text-[13px] [font:inherit] ${activeBucket === tab.type ? "is-active border-b-[#07c05f] font-semibold text-[#07c05f]" : "border-b-transparent text-[#506078] hover:text-[#07c05f]"}`}
            onClick={() => { setActiveBucket((current) => (current === tab.type ? "" : tab.type)); setPage(1); }}
          >
            <span className="wiki-tab-label">{tab.label}</span>
            <span className="wiki-tab-count text-[11px] text-[rgba(0,0,0,0.4)]">{tab.total}</span>
          </button>)}
        </div>
        {tabActions}
      </div>
      {inlineCreating ? <div className="wk-wiki-directory-item flex items-center gap-2 border-b border-line-soft py-[0.55rem]">
        <input className="min-w-0 flex-1 rounded-[6px] border border-[#e7e7e7] px-2 py-1 text-[13px] [font:inherit]" autoFocus
          placeholder={t("wikiBrowser.folderNamePlaceholder")}
          onChange={(event) => setInlineCreatingName(event.target.value)}
          onKeyDown={(event) => { if (event.key === "Enter") void submitInlineCreate(); if (event.key === "Escape") { setInlineCreating(false); setInlineCreatingName(""); } }} />
        <Button type="button" disabled={folderBusy} onClick={() => void submitInlineCreate()}>{t("common.save")}</Button>
        <Button type="button" onClick={() => { setInlineCreating(false); setInlineCreatingName(""); }}>{t("common.cancel")}</Button>
      </div> : null}
      {folderTrail.length > 0 ? <div className="flex items-center gap-1 pb-1 text-[12px] text-[#66758b]">
        <button type="button" className="cursor-pointer border-0 bg-transparent p-0 text-[#66758b] hover:text-[#07c05f] [font:inherit]" onClick={backFolder}>{t("wikiBrowser.backToOverview")}</button>
        {folderTrail.map((crumb) => <span key={crumb.path} className="text-[#344054]">/ {crumb.name}</span>)}
      </div> : null}
      {viewMode === "tree" ? treeRows.map((row) => row.kind === "dir" ? (
        <div key={row.folder.id}
          className="group/wiki-folder flex items-center justify-between gap-2 rounded-[6px] border-0 py-[0.3rem] pl-1 pr-1 hover:bg-[#f0f3f8]"
          style={{ paddingLeft: `${0.25 + row.depth * 0.9}rem` }}
          onDragOver={(event) => { if (canContribute) event.preventDefault(); }}
          onDrop={(event) => { if (canContribute && event.dataTransfer.getData("text/wiki-slug")) { event.preventDefault(); movePageToFolder(event.dataTransfer.getData("text/wiki-slug"), row.folder); } }}>
          {renamingFolderId === row.folder.id ? (
            <input className="min-w-0 flex-1 rounded-[6px] border border-[#e7e7e7] px-2 py-1 text-[13px] [font:inherit]" autoFocus value={renamingName}
              onChange={(event) => setRenamingName(event.target.value)}
              onBlur={() => { void renameFolder(row.folder, renamingName); setRenamingFolderId(""); }}
              onKeyDown={(event) => { if (event.key === "Enter") { void renameFolder(row.folder, renamingName); setRenamingFolderId(""); } if (event.key === "Escape") setRenamingFolderId(""); }} />
          ) : (
            <>
              <button type="button" className="flex min-w-0 flex-1 cursor-pointer items-center gap-[2px] border-0 bg-transparent p-0 text-left text-[13px] text-[#344054] [font:inherit]" onClick={() => toggleDirectory(row.folder)}>
                <span className={`wiki-folder-chevron text-[#98a2b8] transition-transform ${expandedDirs.has(row.folder.path) ? "rotate-90" : ""}`} aria-hidden><WikiGlyph kind="chevron" size={10} /></span>
                <span className="truncate">{row.folder.name}</span>
                <span className="ml-auto shrink-0 pl-1 text-[11px] text-[rgba(0,0,0,0.4)]">{row.folder.page_count}</span>
              </button>
              {canContribute ? <span className="hidden shrink-0 items-center gap-[2px] group-hover/wiki-folder:flex"><Button type="button" aria-label={t("wikiBrowser.renameFolder")} title={t("wikiBrowser.renameFolder")} disabled={folderBusy} onClick={() => startRenameFolder(row.folder)}>✎</Button><Button type="button" aria-label={t("wikiBrowser.deleteFolder")} title={t("wikiBrowser.deleteFolder")} disabled={folderBusy} onClick={() => void deleteFolder(row.folder)}>🗑</Button></span> : null}
            </>
          )}
        </div>
      ) : (
        <button
          className={`wk-wiki-page-item group/wiki-item flex w-full cursor-pointer items-center gap-[6px] rounded-[6px] border-0 bg-transparent px-2 py-[0.45rem] text-left text-[13px] text-[#344054] [font:inherit] transition-colors duration-150 hover:bg-[#f0f3f8] ${selected?.id === row.page.id ? "bg-[#eef4ef]" : ""}`}
          key={row.page.id}
          type="button"
          style={{ paddingLeft: `${0.5 + row.depth * 0.9}rem` }}
          draggable={canContribute}
          onDragStart={(event) => { event.dataTransfer.setData("text/wiki-slug", row.page.slug); event.dataTransfer.effectAllowed = "move"; }}
          onClick={() => choose(row.page)}
        >
          <span className={`wiki-type-dot shrink-0 rounded-full ${WIKI_TYPE_DOT[(row.page as Record<string, unknown>).page_type as string] ?? "bg-[#8c8c8c]"}`} aria-hidden />
          <span className="wk-wiki-page-item-title truncate text-sm leading-5 text-[#202020]">{row.page.title}</span>
        </button>
      )) : null}
    </>
  );

  return (
    <main className="wk-page flex min-h-0 flex-1 flex-col box-border px-8 pt-6 pb-0">
      <header className="wk-header mb-6">
        <DocumentsBreadcrumb
          t={t}
          knowledgeBaseId={knowledgeBaseId}
          kbName={typeof kbMeta?.name === "string" ? kbMeta.name : null}
          kbList={kbList}
          kbMeta={{
            type: typeof kbMeta?.type === "string" ? kbMeta.type : undefined,
            description: typeof kbMeta?.description === "string" ? kbMeta.description : undefined,
            createdAt: typeof kbMeta?.created_at === "string" ? kbMeta.created_at.slice(0, 10) : undefined,
          }}
          supportedFileTypes={supportedFileTypes}
          canManage={canManage}
          onOpenSettings={() => setSettingsOpen(true)}
          tabs={kbTabs}
        />
        <p className="document-subtitle m-0 text-[14px] font-normal leading-[20px] text-[var(--wk-muted,#66758b)]">{t("knowledgeEditor.document.subtitle")}</p>
        <ParserHint
          t={t}
          types={unsupportedFileTypes}
          onConfigure={() => navigate(documentsKBSettingsPath(knowledgeBaseId))}
        />
      </header>
      <div className="wk-wiki-layout flex min-h-0 flex-1 items-stretch gap-[1.25rem] max-[720px]:flex-col">
        <aside className="wk-wiki-sidebar flex w-[300px] shrink-0 flex-col rounded-[10px] border border-[#e7e7e7] bg-white p-[0.9rem] max-[720px]:w-full">
          <div className="wk-wiki-sidebar-header pr-0 pb-2">
            <form className="wk-wiki-search flex items-center gap-[0.45rem] rounded-[8px] border border-[#e7e7e7] bg-white px-[0.6rem] py-[0.45rem] text-[rgba(0,0,0,0.4)]" role="search" aria-label={t("wikiBrowser.page.search")} onSubmit={(event) => { event.preventDefault(); submitSearch(); }}>
              <WikiGlyph kind="search" />
              <Input
                className="border-0 bg-transparent p-0 text-[13px] focus:shadow-none"
                value={searchDraft}
                onChange={(event) => setSearchDraft(event.target.value)}
                placeholder={t("wikiBrowser.searchPlaceholder")}
              />
            </form>
          </div>
          {/* Vue pins the 索引 nav item above the bucket tabs (WikiBrowser.vue:211-217). */}
          <button
            type="button"
            className={`wiki-nav-item flex cursor-pointer items-center gap-[6px] rounded-[6px] border-0 bg-transparent px-2 py-[0.45rem] text-left text-[13px] [font:inherit] ${indexView ? "bg-[#eef4ef] font-semibold text-[#07c05f]" : "text-[#506078] hover:bg-[#f0f3f8]"}`}
            aria-current={indexView ? "page" : undefined}
            onClick={() => void openIndex()}
          >
            <WikiGlyph kind="index" />
            <span className="wiki-nav-text">{t("wikiBrowser.indexTitle")}</span>
          </button>
          <div className="wiki-sidebar-divider my-1 border-t border-[#eef0f4]" aria-hidden />
          <nav className="wk-wiki-page-list flex max-h-[620px] flex-col gap-0.5 overflow-y-auto pr-0.5 pb-3" aria-label={t('wikiBrowser.pageActions')}>
            {directory}
              {viewMode === "list" ? listPages.filter((page) => pageInBucket(page, activeBucket)).map((page) => (
                <button
                  className={`wk-wiki-page-item group/wiki-item flex w-full cursor-pointer items-center gap-[6px] rounded-[6px] border-0 bg-transparent px-2 py-[0.45rem] text-left text-[13px] text-[#344054] [font:inherit] transition-colors duration-150 hover:bg-[#f0f3f8] ${selected?.id === page.id ? "bg-[#eef4ef]" : ""}`}
                  key={page.id}
                  type="button"
                  draggable={canContribute}
                  onDragStart={(event) => { event.dataTransfer.setData("text/wiki-slug", page.slug); event.dataTransfer.effectAllowed = "move"; }}
                  onClick={() => choose(page)}
                >
                  <span className={`wiki-type-dot shrink-0 rounded-full ${WIKI_TYPE_DOT[(page as Record<string, unknown>).page_type as string] ?? "bg-[#8c8c8c]"}`} aria-hidden />
                  <span className="wk-wiki-page-item-title truncate text-sm leading-5 text-[#202020]">{page.title}</span>
                </button>
              )) : null}
              {state.status === "loading" ? (
                <Status>{t("wikiBrowser.loading")}</Status>
              ) : null}
              {state.status === "error" ? (
                <Status tone="error">{state.message}</Status>
              ) : null}
              {state.status === "success" && listPages.length === 0 ? (
                <div className="wk-wiki-empty flex flex-1 flex-col items-center gap-2 px-5 py-[60px] text-center text-[rgba(0,0,0,0.6)]">
                  <span className="wk-wiki-empty-icon text-[36px] leading-none text-[#07c05f]" aria-hidden="true">
                    ▧
                  </span>
                  <strong>{keyword ? t("wikiBrowser.searchNoResults") : t("wikiBrowser.emptyTitle")}</strong>
                  {!keyword ? <span>{t("wikiBrowser.emptyDesc")}</span> : null}
                </div>
              ) : null}
            </nav>
          </aside>
        <section className="wk-wiki-reader-pane flex min-w-0 flex-1 flex-col rounded-[10px] border border-[#e7e7e7] bg-white p-[1.4rem] max-[720px]:p-4">
          {!selected && !editing && !indexView ? (() => {
            const emptyState = wikiReaderEmptyState(t, pages.length > 0);
            return <div className="wk-wiki-reader-empty flex min-h-[22rem] min-w-0 flex-col items-center justify-center gap-2 px-5 py-[60px] text-center text-[rgba(0,0,0,0.6)]">
              <span className="wk-wiki-empty-icon text-[36px] leading-none text-[#07c05f]" aria-hidden="true">▧</span>
              <strong>{emptyState.title}</strong>
              {emptyState.description ? <span>{emptyState.description}</span> : null}
            </div>;
          })() : null}
          {indexView ? (
            <article className="wk-wiki-reader min-w-0" aria-label={t("wikiBrowser.indexTitle")}>
              <div className="wk-header mb-6 flex items-start justify-between gap-4">
                <div>
                  <h2 className="m-0 text-[#07c05f]">{t("wikiBrowser.indexTitle")}</h2>
                  <p className="m-0 mt-[6px] inline-block rounded-[4px] border border-[#e4e7ec] bg-[#f6f8fa] px-[6px] py-[2px] text-[11px] leading-[16px] text-[#66758b]">{t("wikiBrowser.indexOverviewTag")}</p>
                </div>
              </div>
              <WikiIndexView
                indexView={indexView}
                loading={indexLoading}
                error={indexError}
                hasMore={indexNextCursor !== null}
                labelFor={(type) => {
                  const key = WIKI_INDEX_TYPE_LABEL_KEYS[type];
                  return key ? t(key) : type;
                }}
                onLoadMore={() => void loadMoreIndex()}
                onNavigate={navigateToSlug}
                onOpenImage={(src) => setPreviewSrc(src)}
              />
            </article>
          ) : null}
          {selected && !editing ? (
            <article className="wk-wiki-reader min-w-0" aria-label={selected.title}>
              <div className="wk-header mb-6 flex items-start justify-between gap-4">
                <div>
                  <h2>{selected.title}</h2>
                  <p className="wk-muted text-muted">{selected.summary || "—"}</p>
                </div>
                <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
                  {canContribute ? <Button type="button" onClick={() => setEditing(true)}>
                    {t("wikiBrowser.editBtn")}
                  </Button> : null}
                  <Button type="button" onClick={() => void openHistory()}>
                    {t("wikiBrowser.historyBtn")}
                  </Button>
                  {canContribute ? <Button
                    type="button"
                    loading={deleteBusy}
                    onClick={() => void deleteWikiPage()}
                  >
                    {t("wikiBrowser.deletePageBtn")}
                  </Button> : null}
                </div>
              </div>
              {saveState?.status === "error" ? <Status tone="error">{saveState.message}</Status> : null}
              {(() => {
                // Vue renderedContent computed: strip the duplicate leading
                // H1, then run the same wiki-link → marked → DOMPurify chain.
                const body = stripDuplicateLeadingTitle(selected.content || "", selected.title);
                const rendered = renderWikiMarkdown(body, {
                  resolveSlugName: (nextSlug) => wikiSlugDisplayName(nextSlug, pages),
                });
                return (
                  <div
                    className="wk-wiki-reader-content wiki-reader-body m-0 box-border min-h-[22rem]"
                    onClick={(event) => handleWikiBodyClick(event, { navigate: navigateToSlug, openImage: setPreviewSrc })}
                    dangerouslySetInnerHTML={{ __html: rendered }}
                  />
                );
              })()}
              <WikiReaderFooter
                page={selected}
                resolveSlugName={(nextSlug) => wikiSlugDisplayName(nextSlug, pages)}
                translate={t}
                onNavigate={navigateToSlug}
                onOpenSourceDoc={onOpenSourceDoc}
                sourceTitles={sourceTitles}
              />
            </article>
          ) : null}
          {canContribute ? <form
            className="wk-wiki-editor grid gap-[0.7rem]"
            // The form surfaces only in edit/create mode (Vue opens the editor
            // from 编辑/新建 actions); the index/reader views never show it.
            style={{ display: editing ? undefined : "none" }}
            onSubmit={save}
          >
            <label>
              {t("wikiBrowser.newPageTitleLabel")}{" "}
              <Input
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                required
              />
            </label>
            <label>
              {t("wikiBrowser.newPageSlugLabel")}{" "}
              <Input
                value={slug}
                onChange={(event) => setSlug(event.target.value)}
                required
                disabled={Boolean(selected)}
              />
            </label>
            <label>
              {t("wikiBrowser.editSummaryPlaceholder")}{" "}
              <Input
                value={summary}
                onChange={(event) => setSummary(event.target.value)}
              />
            </label>
            <label>
              {t("wikiBrowser.newPageContentLabel")}{" "}
              <Textarea
                value={content}
                onChange={(event) => setContent(event.target.value)}
                rows={14}
                required
              />
            </label>
            <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
              <span className="mr-auto text-[0.85rem] text-muted">{selected ? t("wikiBrowser.version", { ver: version }) : t("wikiBrowser.page.new")}</span>
              <Button type="submit">
                {selected ? t("wikiBrowser.editSave") : t("wikiBrowser.page.new")}
              </Button>
              {selected ? (
                <>
                  <Button type="button" onClick={cancelEdit}>
                    {t("common.cancel")}
                  </Button>
                  <Button type="button" onClick={() => void reloadSelected()}>
                    {t("wikiBrowser.editConflictReload")}
                  </Button>
                  <Button type="button" onClick={() => void openHistory()}>
                    {t("wikiBrowser.historyBtn")}
                  </Button>
                </>
              ) : null}
            </div>
            {saveState?.status === "conflict" ? (
              <div className="wk-wiki-conflict flex items-center justify-between gap-[0.5rem]">
                <Status tone="warning">{saveState.message}</Status>
                <Button type="button" onClick={() => void overwriteConflict()}>
                  {t("wikiBrowser.editConflictOverwrite")}
                </Button>
              </div>
            ) : null}
            {saveState?.status === "error" ? (
              <Status tone="error">{saveState.message}</Status>
            ) : null}
            {saveState?.status === "saved" ? (
                <Status tone="success">{t("wikiBrowser.editSaveSuccess")}</Status>
            ) : null}
          </form> : null}
        </section>
      </div>
      {pager.total > WIKI_PAGE_SIZE ? (
        <nav
          className="wk-pagination flex items-center justify-center gap-3 pt-4"
          aria-label={t("wikiBrowser.page.title")}
        >
          <Button
            type="button"
            disabled={!pager.hasPrevious}
            onClick={() => setPage((value) => value - 1)}
          >
            {t("wikiBrowser.page.previous")}
          </Button>
          <span>
            {t("wikiBrowser.page.page", {
              page: pager.page,
              total: pager.total,
            })}
          </span>
          <Button
            type="button"
            disabled={!pager.hasNext}
            onClick={() => setPage((value) => value + 1)}
          >
            {t("wikiBrowser.page.next")}
          </Button>
        </nav>
      ) : null}
      {settingsOpen ? (
        <Dialog
          open
          title={t('knowledgeBase.settings')}
          closeLabel={t('common.close')}
          onClose={() => setSettingsOpen(false)}
          className="h-[min(85vh,750px)] w-[min(1000px,90vw)]! max-h-[min(750px,85vh)]! overflow-auto"
        >
          <KnowledgeSettingsPage client={client} knowledgeBaseId={knowledgeBaseId} role={canManage ? 'admin' : 'viewer'} />
        </Dialog>
      ) : null}
      {previewSrc ? <WikiImagePreview src={previewSrc} onClose={() => setPreviewSrc(null)} /> : null}
      {historyOpen && selected ? (
        <Card className="wk-wiki-history mt-4">
          <div className="wk-header mb-6 flex items-start justify-between gap-4">
            <div>
              <h2>{t("wikiBrowser.historyTitle", { title: selected.title })}</h2>
              <p className="wk-muted text-muted">{t("wikiBrowser.revisionCurrentHint")}</p>
            </div>
            <Button type="button" onClick={() => setHistoryOpen(false)}>
              {t("common.close")}
            </Button>
          </div>
          {historyError ? <Status tone="error">{historyError}</Status> : null}
          {historyLoading && !revision ? (
            <Status>{t("wikiBrowser.loading")}</Status>
          ) : null}
          {!historyLoading && revisions.length === 0 ? (
            <Status>{t("wikiBrowser.revisionEmpty")}</Status>
          ) : null}
          <div className="wk-wiki-history-layout grid grid-cols-[minmax(180px,260px)_1fr] gap-5 max-[720px]:grid-cols-1">
            <nav aria-label={t('wikiBrowser.historyBtn')}>
              <ul className="wk-list m-0 list-none p-0">
                {revisions.map((item) => (
                  <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]">
                    <button
                      className="border-0 bg-transparent cursor-pointer p-0 text-left text-primary-deep [font:inherit] [font-weight:650]! hover:underline"
                      type="button"
                      onClick={() => void chooseRevision(item)}
                    >
                      {t("wikiBrowser.version", { ver: item.version })}
                    </button>
                    <span className="font-mono text-[0.8rem] text-muted">{item.edit_source ?? "user"}</span>
                  </li>
                ))}
              </ul>
            </nav>
            <div>
              {revision ? (
                <>
                  <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
                    <strong>
                      v{revision.version} → v{selected.version}
                    </strong>
                    {canContribute ? <Button
                      type="button"
                      onClick={() => void revertRevision()}
                      loading={reverting}
                    >
                      {t("wikiBrowser.revertBtn")}
                    </Button> : null}
                  </div>
                  {revisionDiff.length === 0 ? (
                    <Status>{t("wikiBrowser.revisionDiffEmpty")}</Status>
                  ) : (
                    <div className="wk-diff grid gap-3">
                      {revisionDiff.map((section) => (
                        <section key={section.field}>
                          <h3>{section.field}</h3>
                          <pre>
                            {section.lines.map((line, index) => (
                              <span
                                key={`${section.field}-${index}`}
                                className={`wk-diff-${line.type} block ${line.type === "add" ? "bg-[#ecfdf3] text-[#137333]" : line.type === "del" ? "bg-[#fef3f2] text-[#b42318]" : ""}`}
                              >
                                {line.type === "add"
                                  ? "+"
                                  : line.type === "del"
                                    ? "-"
                                    : " "}
                                {line.text}
                                {"\n"}
                              </span>
                            ))}
                          </pre>
                        </section>
                      ))}
                    </div>
                  )}
                </>
              ) : (
                <Status>{t("wikiBrowser.revisionSelectHint")}</Status>
              )}
            </div>
          </div>
        </Card>
      ) : null}
    </main>
  );
}
