import { useEffect, useMemo, useRef, useState } from "react";
import type {
  WikiPage as WikiPageModel,
  WikiPageRevision,
  WikiFolderNode,
  WikiIndexResponse,
  WeKnoraClient,
} from "@weknora/api-client";
import { diffWikiRevision } from "@weknora/domain/wiki/diff";
import { findSharedKBGrant } from "../wiki/edit-permission.ts";
// S6 换装（T15 前置）：packages/ui 旧栈 离栈——Button/Input/Textarea 换 tdesign
// （playbook §1：onChange 改 (value) 签名）；Dialog 是测试锚定弹层
// （[role="dialog"]/.wk-dialog-close）且宿主 KnowledgeSettingsPage 依赖
// .wk-dialog* DOM，走 shared/wk-legacy WkDialog；Card/Status 同走 wk-legacy。
import { Button as TButton, Input as TdInput, Textarea as TdTextarea } from "tdesign-react";
import { WkCard as Card, WkDialog as Dialog, WkStatus as Status } from "../shared/wk-legacy.tsx";
import { Icon as TIcon } from "tdesign-icons-react";
import { DocumentsBreadcrumb, ParserHint, type DocumentsBreadcrumbTab, type KBChromeListItem } from "../documents/DocumentsPageChrome.tsx";
import type { KBInfoPopoverKB } from "../documents/KBInfoPopover.tsx";
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
import "./wiki-u.css";
import "./wiki-reader.css";
import "../documents/documents.td.css";
import { createTranslator, useAppLocale } from "../i18n.ts";
import { navigate } from "../platform/navigation.ts";
import { pagerState } from "../pagination.ts";
import { wikiEditPermission } from "./edit-permission.ts";
import { canUploadKnowledgeDocuments, resolveKBSurfaceTabs, type KBSurfaceKB, type KBSurfaceMe, type KBSurfaceTab } from "../knowledge/permissions.ts";

const WIKI_PAGE_SIZE = 50;

// Vue renders every sidebar icon as an inline t-icon SVG, so icon glyphs
// never appear in the accessible text. The React port mirrors that with a
// small stroke-icon set (aria-hidden, no text content); the path data is
// lifted from TDesign's 24×24 outline icons (stroke 2, square caps) so the
// glyph shapes match the Vue screenshot pixel-for-pixel at 15/16px.
export function WikiGlyph({ kind, size = 14 }: { kind: "search" | "index" | "tree" | "list" | "folder-add" | "page-add" | "chevron" | "tag" | "lightbulb" | "relativity" | "view-module" | "file" | "browse"; size?: number }) {
  const stroke = {
    stroke: "currentColor",
    strokeWidth: 2,
    strokeLinecap: "square" as const,
    strokeLinejoin: "miter" as const,
    fill: "none",
  };
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {/* t-icon-search */}
      {kind === "search" ? <g {...stroke}><path d="M15.8033 15.8033C12.8744 18.7322 8.12563 18.7322 5.1967 15.8033C2.26777 12.8744 2.26777 8.12563 5.1967 5.1967C8.12563 2.26777 12.8744 2.26777 15.8033 5.1967C18.7322 8.12563 18.7322 12.8744 15.8033 15.8033Z" /><path d="M15.8027 15.8037L21.106 21.107" /></g> : null}
      {/* t-icon-catalog */}
      {kind === "index" ? <g {...stroke}><path d="M4 2H20V22H4V2Z" /><path d="M9 8H15M9 12H15M9 16H15" /></g> : null}
      {/* t-icon-tree-list */}
      {kind === "tree" ? <g {...stroke}><circle cx="6" cy="6" r="3" /><circle cx="6" cy="18" r="3" /><path d="M6 9V15" /><path d="M21 12L13 12M18 5L13 5M18 19H13" /></g> : null}
      {/* t-icon-view-list */}
      {kind === "list" ? <g {...stroke}><path d="M3 5H21M3 12H21M3 19H21" /></g> : null}
      {/* t-icon-folder-add */}
      {kind === "folder-add" ? <g {...stroke}><path d="M22 11V6H11L9 3.5L2 3.5L2 20H13" /><path d="M20 15V18M20 18V21M20 18H17M20 18H23" /></g> : null}
      {/* t-icon-file-add */}
      {kind === "page-add" ? <g {...stroke}><path d="M20 11V7L15 2H4V22H11M14 2V8H20" /><path d="M19 15V19M19 19V23M19 19H15M19 19H23" /></g> : null}
      {/* t-icon-chevron-right */}
      {kind === "chevron" ? <g {...stroke} strokeLinecap="round"><path d="M9.5 17.5L15 12L9.5 6.5" /></g> : null}
      {/* t-icon-tag (getPageIcon: entity) */}
      {kind === "tag" ? <g {...stroke}><path d="M10.8788 21.6066L2.39355 13.1214L11.5149 4.01475L20.0002 4L20.0002 12.5L10.8788 21.6066Z" /><path d="M15.9966 7.99976H16.0005L16.0005 8.00366L15.9966 8.00366V7.99976Z" /></g> : null}
      {/* t-icon-lightbulb (getPageIcon: concept) */}
      {kind === "lightbulb" ? <g {...stroke}><path d="M11.9998 0.999879C15.9698 1.0019 19.1998 4.23028 19.1998 8.19988C19.1998 9.28948 18.9833 10.1185 18.4926 11.3071C18.0019 12.4956 16.7461 15.0037 16.0039 16.9999H7.99609C7.28648 14.9986 6.00366 12.4962 5.5086 11.3071C5.01355 10.1179 4.7998 9.28948 4.7998 8.19988C4.7998 4.23028 8.02981 0.997856 11.9998 0.999879Z" /><path d="M8.5 20H15.5M10.5 23H13.5" /></g> : null}
      {/* t-icon-relativity (getPageIcon: synthesis) */}
      {kind === "relativity" ? <g {...stroke}><path d="M3 3H14V14H3V3Z" /><path d="M10 10H21V21H10V10Z" /></g> : null}
      {/* t-icon-view-module (getPageIcon: comparison) */}
      {kind === "view-module" ? <g {...stroke}><path d="M2 20H22V4H2V20Z" /><path d="M8.6665 4V20M15.333 4V20M2 12H22" /></g> : null}
      {/* t-icon-file (getPageIcon fallback + summary) */}
      {kind === "file" ? <g {...stroke}><path d="M14 2V8H20M14 2H15L20 7V8M14 2H4V22H20V8" /></g> : null}
      {/* t-icon-browse (reader empty state) */}
      {kind === "browse" ? <g {...stroke}><path d="M11.9997 4C6.86881 4 2.52275 7.36017 1.04199 12C2.52275 16.6398 6.86881 20 11.9997 20C17.1306 20 21.4766 16.6398 22.9574 12C21.4766 7.36017 17.1306 4 11.9997 4Z" /><path d="M16 12C16 14.2091 14.2091 16 12 16C9.79086 16 8 14.2091 8 12C8 9.79086 9.79086 8 12 8C14.2091 8 16 9.79086 16 12Z" /></g> : null}
    </svg>
  );
}

// Vue getPageIcon (WikiBrowser.vue:1980-1990) gives each page_type its own
// 15px placeholder-gray glyph in the sidebar tree/list rows.
function WikiPageTypeGlyph({ pageType }: { pageType: string }) {
  const kind = pageType === "entity" ? "tag"
    : pageType === "concept" ? "lightbulb"
    : pageType === "synthesis" ? "relativity"
    : pageType === "comparison" ? "view-module"
    : "file";
  return (
    <span className="wk-wiki-1" aria-hidden="true">
      <WikiGlyph kind={kind} size={15} />
    </span>
  );
}

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
      className="wk-wiki-img-preview wk-wiki-2"
      role="dialog"
      aria-modal="true"
      onClick={(event) => {
        // t-image-viewer closeOnOverlay: only a mask click closes, not
        // clicks on the image or the toolbar.
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="wk-wiki-img-preview-mask wk-wiki-3 wk-wiki-mask-85" aria-hidden="true" />
      <img
        className="wk-wiki-img-preview-image wk-wiki-4"
        src={src}
        alt=""
        style={{ transform: `scale(${scale})` }}
        draggable={false}
      />
      <div className="wk-wiki-img-preview-toolbar wk-wiki-5 wk-wiki-mask-60">
        <button
          type="button"
          className="wk-wiki-img-preview-zoom-out wk-wiki-6"
          onClick={() => setScale((value) => wikiPreviewStep(value, -0.25))}
        >
          −
        </button>
        <span className="wk-wiki-img-preview-scale wk-wiki-7">
          {Math.round(scale * 100)}%
        </span>
        <button
          type="button"
          className="wk-wiki-img-preview-zoom-in wk-wiki-6"
          onClick={() => setScale((value) => wikiPreviewStep(value, 0.25))}
        >
          +
        </button>
      </div>
      <button
        type="button"
        className="wk-wiki-img-preview-close wk-wiki-8 wk-wiki-mask-60"
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
        className="wiki-reader-body wiki-index-body wk-wiki-9"
        onClick={(event) => handleWikiBodyClick(event, { navigate: onNavigate, openImage: onOpenImage })}
        dangerouslySetInnerHTML={{
          __html: renderWikiMarkdown(markdown, { resolveSlugName: (slug) => slug }),
        }}
      />
      {hasMore ? (
        <div className="wiki-index-sentinel wk-wiki-10">
          <TButton type="button" disabled={loading} onClick={onLoadMore}>
            {loading ? t("wikiBrowser.loading") : t("wikiBrowser.loadMoreShort")}
          </TButton>
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
  // Vue create dialog state (WikiBrowser.vue L2906-2915): creation is a
  // t-dialog「新建 Wiki 页面」 with its own form model — title / slug /
  // pageType (concept default) / content — plus createPageSlugTouched. The
  // dialog derives the slug from "<type>/<slugified-title>" until the user
  // edits it. The inline editor below only serves page EDIT mode.
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState({ title: "", slug: "", pageType: "concept", content: "" });
  const [createSlugTouched, setCreateSlugTouched] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [createBusy, setCreateBusy] = useState(false);
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
  // KBInfoPopover 访问段（B2 批 2）：Vue authStore user id + orgStore share 行。
  const [meId, setMeId] = useState("");
  const [infoSharedKb, setInfoSharedKb] = useState<{ orgName: string; sharedAt: string } | null>(null);
  const [infoPermission, setInfoPermission] = useState("");
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
      const meRow = me as { user?: { id?: unknown } } | null;
      setMeId(meRow?.user && typeof meRow.user.id === 'string' ? meRow.user.id : '');
      const grant = findSharedKBGrant(sharedRows, knowledgeBaseId);
      const row = Array.isArray(sharedRows)
        ? (sharedRows as Array<Record<string, unknown>>).find((entry) => {
          const entryKb = entry?.knowledge_base as { id?: unknown } | null | undefined;
          return entryKb && String(entryKb.id) === knowledgeBaseId;
        })
        : null;
      setInfoSharedKb(grant && row ? {
        orgName: typeof row.org_name === "string" ? row.org_name : "",
        sharedAt: typeof row.shared_at === "string" ? row.shared_at : "",
      } : null);
      setInfoPermission(grant?.permission ?? "");
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
    // Vue openCreatePageDialog (WikiBrowser.vue L3012-3016): re-seed the
    // create form and show the dialog; the selected page / current view stays
    // untouched behind the modal.
    setCreateForm({ title: "", slug: "", pageType: "concept", content: "" });
    setCreateSlugTouched(false);
    setCreateError(null);
    setCreateOpen(true);
  }

  // Vue syncCreatePageSlug (WikiBrowser.vue L3022-3031): derive
  // "<type>/<slugified-title>" from the title while the user has not touched
  // the slug field themselves. Only ASCII-ish titles produce a usable base;
  // otherwise the user types one.
  function onTitleInputForCreate(nextTitle: string) {
    setCreateForm((current) => {
      if (createSlugTouched) return { ...current, title: nextTitle };
      const base = nextTitle
        .toLowerCase()
        .trim()
        .replace(/[^a-z0-9\s_-]/g, "")
        .replace(/\s+/g, "-")
        .replace(/-{2,}/g, "-")
        .replace(/^-|-$/g, "");
      return { ...current, title: nextTitle, slug: base ? `${current.pageType}/${base}` : "" };
    });
  }

  // Vue t-dialog cancel (v-model:visible=false): drop the dialog without
  // posting anything. The draft is re-seeded on the next open.
  function cancelCreate() {
    setCreateOpen(false);
    setCreateError(null);
  }

  // Vue submitCreatePage (WikiBrowser.vue L3033-3061): title + slug are
  // required (content stays optional), the slug pattern gates the shape, the
  // payload posts { slug, title, page_type, content } — no summary on create —
  // and a successful create closes the dialog.
  async function submitCreate(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canContribute || createBusy) return;
    const trimmedTitle = createForm.title.trim();
    const trimmedSlug = createForm.slug.trim().replace(/^\/+|\/+$/g, "");
    if (!trimmedTitle || !trimmedSlug) {
      setCreateError(t("wikiBrowser.newPageMissingFields"));
      return;
    }
    if (!/^[\p{L}\p{N}][\p{L}\p{N}\s_\-/]*$/u.test(trimmedSlug)) {
      setCreateError(t("wikiBrowser.newPageSlugHint"));
      return;
    }
    setCreateBusy(true);
    try {
      const page = await client.wiki.create(knowledgeBaseId, {
        slug: trimmedSlug,
        title: trimmedTitle,
        page_type: createForm.pageType,
        content: createForm.content,
      });
      setCreateOpen(false);
      setCreateError(null);
      choose(page);
      setSaveState({ status: "saved", page });
      await loadPages();
    } catch (error) {
      setCreateError(
        error instanceof Error
          ? error.message
          : t("wikiBrowser.newPageFailed"),
      );
    } finally {
      setCreateBusy(false);
    }
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canContribute || !selected) return;
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
  // Vue .wiki-view-toggle: 2px-padded 1px-stroke pill, two 24×22 buttons
  // (radius 4); the active button gets the brand tint + a hairline shadow.
  const viewToggle = (
    <div className="wiki-view-toggle wk-wiki-11" role="group" aria-label={t("wikiBrowser.viewModeToggle")}>
      <button type="button" className={`wk-wiki-66 ${viewMode === "tree" ? "wk-wiki-67" : "wk-wiki-68"}`} aria-pressed={viewMode === "tree"} aria-label={t("wikiBrowser.viewTree")} title={t("wikiBrowser.viewTree")} onClick={() => switchViewMode("tree")}><TIcon name="tree-list" size="15px" /></button>
      <button type="button" className={`wk-wiki-66 ${viewMode === "list" ? "wk-wiki-67" : "wk-wiki-68"}`} aria-pressed={viewMode === "list"} aria-label={t("wikiBrowser.viewList")} title={t("wikiBrowser.viewList")} onClick={() => switchViewMode("list")}><TIcon name="view-list" size="15px" /></button>
    </div>
  );
  // Vue .wiki-tab-bar-action: borderless 26×26 icon buttons (15px icon).
  const tabAction = (props: { label: string; disabled?: boolean; onClick: () => void; kind: "folder-add" | "page-add" }) => (
    <button
      type="button"
      className="wiki-tab-bar-action wk-wiki-12"
      disabled={props.disabled}
      aria-label={props.label}
      title={props.label}
      onClick={props.onClick}
    >
      <WikiGlyph kind={props.kind} size={15} />
    </button>
  );
  const tabActions = (
    <div className="wiki-tab-bar-actions wk-wiki-13">
      {viewToggle}
      {canContribute ? tabAction({ label: t("wikiBrowser.newRootFolder"), disabled: folderBusy, onClick: startInlineCreate, kind: "folder-add" }) : null}
      {canContribute ? tabAction({ label: t("wikiBrowser.newPageBtn"), onClick: newPage, kind: "page-add" }) : null}
    </div>
  );
  const directory = (
    <>
      {/* Vue .wiki-tab-bar: sticky 4px/6px padded row; tabs are 13px text with
          a 2px brand underline inset 2px (::after) rather than a full border. */}
      <div className="wiki-tab-bar wk-wiki-14" role="tablist" aria-label={t("wikiBrowser.viewModeToggle")}>
        <div className="wiki-tab-bar-scroll wk-wiki-15 wk-wiki-noscrollbar">
          {bucketTabs.map((tab) => <button
            key={tab.type}
            type="button"
            role="tab"
            aria-selected={activeBucket === tab.type}
            className={`wiki-tab wk-wiki-69 ${activeBucket === tab.type ? "is-active wk-wiki-70" : "wk-wiki-71"}`}
            onClick={() => { setActiveBucket((current) => (current === tab.type ? "" : tab.type)); setPage(1); }}
          >
            <span className={`wiki-tab-label wk-wiki-72 ${activeBucket === tab.type ? "wk-wiki-73" : "wk-wiki-74"}`}>{tab.label}</span>
            <span className={`wiki-tab-count wk-wiki-75 ${activeBucket === tab.type ? "wk-wiki-76" : "wk-wiki-77"}`}>{tab.total}</span>
          </button>)}
        </div>
        {tabActions}
      </div>
      {inlineCreating ? <div className="wk-wiki-directory-item wk-wiki-16">
        <input className="wk-wiki-17" autoFocus
          placeholder={t("wikiBrowser.folderNamePlaceholder")}
          onChange={(event) => setInlineCreatingName(event.target.value)}
          onKeyDown={(event) => { if (event.key === "Enter") void submitInlineCreate(); if (event.key === "Escape") { setInlineCreating(false); setInlineCreatingName(""); } }} />
        <TButton type="button" disabled={folderBusy} onClick={() => void submitInlineCreate()}>{t("common.save")}</TButton>
        <TButton type="button" onClick={() => { setInlineCreating(false); setInlineCreatingName(""); }}>{t("common.cancel")}</TButton>
      </div> : null}
      {folderTrail.length > 0 ? <div className="wk-wiki-18">
        <button type="button" className="wk-wiki-19" onClick={backFolder}>{t("wikiBrowser.backToOverview")}</button>
        {folderTrail.map((crumb) => <span key={crumb.path} className="wk-wiki-20">/ {crumb.name}</span>)}
      </div> : null}
      {viewMode === "tree" ? treeRows.map((row) => row.kind === "dir" ? (
        <div key={row.folder.id}
          className="wk-wiki-folder-row wk-wiki-21"
          style={{ paddingLeft: `${(10 + row.depth * 14) / 16}rem` }}
          onDragOver={(event) => { if (canContribute) event.preventDefault(); }}
          onDrop={(event) => { if (canContribute && event.dataTransfer.getData("text/wiki-slug")) { event.preventDefault(); movePageToFolder(event.dataTransfer.getData("text/wiki-slug"), row.folder); } }}>
          {renamingFolderId === row.folder.id ? (
            <input className="wk-wiki-17" autoFocus value={renamingName}
              onChange={(event) => setRenamingName(event.target.value)}
              onBlur={() => { void renameFolder(row.folder, renamingName); setRenamingFolderId(""); }}
              onKeyDown={(event) => { if (event.key === "Enter") { void renameFolder(row.folder, renamingName); setRenamingFolderId(""); } if (event.key === "Escape") setRenamingFolderId(""); }} />
          ) : (
            <>
              <button type="button" className="wk-wiki-22" onClick={() => toggleDirectory(row.folder)}>
                <span className={`wiki-folder-chevron wk-wiki-78 ${expandedDirs.has(row.folder.path) ? "wk-wiki-79" : ""}`} aria-hidden><TIcon name="chevron-right" size="15px" /></span>
                <span className="wiki-directory-title wk-wiki-23">{row.folder.name}</span>
                {/* Vue reserves a 15px hover-reveal slot after the count
                    (wiki-directory-action--reveal), so the count sits 17px in
                    from the row's right inset. */}
                <span className="wk-wiki-24">{row.folder.page_count}</span>
              </button>
              {canContribute ? <span className="wk-wiki-folder-actions wk-wiki-25"><TButton type="button" aria-label={t("wikiBrowser.renameFolder")} title={t("wikiBrowser.renameFolder")} disabled={folderBusy} onClick={() => startRenameFolder(row.folder)}>✎</TButton><TButton type="button" aria-label={t("wikiBrowser.deleteFolder")} title={t("wikiBrowser.deleteFolder")} disabled={folderBusy} onClick={() => void deleteFolder(row.folder)}>🗑</TButton></span> : null}
            </>
          )}
        </div>
      ) : (
        <button
          className={`wk-wiki-page-item wk-wiki-80 ${selected?.id === row.page.id ? "wk-wiki-81" : ""}`}
          key={row.page.id}
          type="button"
          style={{ paddingLeft: `${(10 + row.depth * 14) / 16}rem` }}
          draggable={canContribute}
          onDragStart={(event) => { event.dataTransfer.setData("text/wiki-slug", row.page.slug); event.dataTransfer.effectAllowed = "move"; }}
          onClick={() => choose(row.page)}
        >
          <WikiPageTypeGlyph pageType={String((row.page as Record<string, unknown>).page_type ?? "")} />
          <span className="wk-wiki-page-item-title wk-wiki-26">{row.page.title}</span>
        </button>
      )) : null}
    </>
  );

  return (
    /* Vue KnowledgeBase.vue page shell: .knowledge-layout（margin 0 16px 0 4px
       + padding 24px 32px 0 + gap 20px）> .document-header + .wiki-main-area；
       样式平移在 documents/documents.td.css（breadcrumb/容器同源共享）。 */
    <div className="knowledge-layout">
      <div className="document-header">
        <div className="document-header-title">
          <DocumentsBreadcrumb
            t={t}
            knowledgeBaseId={knowledgeBaseId}
            kbName={typeof kbMeta?.name === "string" ? kbMeta.name : null}
            kbList={kbList}
            kbInfo={kbMeta as unknown as KBInfoPopoverKB | null}
            infoUserId={meId}
            infoSharedKb={infoSharedKb}
            infoPermission={infoPermission}
            supportedFileTypes={supportedFileTypes}
            canManage={canManage}
            onOpenSettings={() => setSettingsOpen(true)}
            tabs={kbTabs}
          />
          <p className="document-subtitle">{t("knowledgeEditor.document.subtitle")}</p>
          <ParserHint
            t={t}
            types={unsupportedFileTypes}
            onConfigure={() => navigate(documentsKBSettingsPath(knowledgeBaseId))}
          />
        </div>
      </div>
      {/* Vue .wiki-main-area：flex:1 + overflow hidden；.wiki-browser 全出血白底行。 */}
      <div className="wiki-main-area">
      <div className="wk-wiki-layout wk-wiki-27">
        <aside className="wk-wiki-sidebar wk-wiki-28">
          <div className="wk-wiki-sidebar-header wk-wiki-29">
            {/* Vue sidebar search = a single t-input（prefix t-icon-search sprite，
                border #dcdcdc、radius 3）；几何由平移 CSS .wk-wiki-search-input 段承载。 */}
            <form className="wk-wiki-search" role="search" aria-label={t("wikiBrowser.page.search")} onSubmit={(event) => { event.preventDefault(); submitSearch(); }}>
              <TdInput
                className="wk-wiki-search-input"
                value={searchDraft}
                onChange={(value: unknown) => setSearchDraft(String(value ?? ""))}
                clearable
                placeholder={t("wikiBrowser.searchPlaceholder")}
                prefixIcon={<TIcon name="search" />}
                onEnter={() => submitSearch()}
                onClear={() => setSearchDraft("")}
              />
            </form>
          </div>
          {/* Vue pins the 索引 nav item above the bucket tabs (WikiBrowser.vue:211-217):
              min-height 30px row, 10px inset, 16px icon, 14px/20px text; the
              active state is a #f3f3f3 pill with brand-tinted icon+text. */}
          <button
            type="button"
            className={`wiki-nav-item wk-wiki-82 ${indexView ? "wk-wiki-83" : "wk-wiki-84"}`}
            aria-current={indexView ? "page" : undefined}
            onClick={() => void openIndex()}
          >
            <WikiGlyph kind="index" size={16} />
            <span className="wiki-nav-text wk-wiki-30">{t("wikiBrowser.indexTitle")}</span>
          </button>
          <div className="wiki-sidebar-divider wk-wiki-31" aria-hidden />
          <nav className="wk-wiki-page-list wk-wiki-32 wk-wiki-noscrollbar" aria-label={t('wikiBrowser.pageActions')}>
            {directory}
              {viewMode === "list" ? listPages.filter((page) => pageInBucket(page, activeBucket)).map((page) => (
                <button
                  className={`wk-wiki-page-item wk-wiki-85 ${selected?.id === page.id ? "wk-wiki-81" : ""}`}
                  key={page.id}
                  type="button"
                  draggable={canContribute}
                  onDragStart={(event) => { event.dataTransfer.setData("text/wiki-slug", page.slug); event.dataTransfer.effectAllowed = "move"; }}
                  onClick={() => choose(page)}
                >
                  <WikiPageTypeGlyph pageType={String((page as Record<string, unknown>).page_type ?? "")} />
                  <span className="wk-wiki-page-item-title wk-wiki-33">{page.title}</span>
                </button>
              )) : null}
              {state.status === "loading" ? (
                <Status>{t("wikiBrowser.loading")}</Status>
              ) : null}
              {state.status === "error" ? (
                <Status tone="error">{state.message}</Status>
              ) : null}
              {state.status === "success" && listPages.length === 0 ? (
                <div className="wk-wiki-empty wk-wiki-34">
                  <span className="wk-wiki-empty-icon wk-wiki-35" aria-hidden="true">
                    <WikiGlyph kind="file" size={36} />
                  </span>
                  <strong className="wk-wiki-36">{keyword ? t("wikiBrowser.searchNoResults") : t("wikiBrowser.emptyTitle")}</strong>
                  {!keyword ? <span className="wk-wiki-37">{t("wikiBrowser.emptyDesc")}</span> : null}
                </div>
              ) : null}
            </nav>
          </aside>
        {/* Vue .wiki-content + .wiki-reader: no card chrome; the reader
            scrolls with 24px side / 16px bottom padding. */}
        <section className="wk-wiki-reader-pane wk-wiki-38">
          {!selected && !editing && !indexView ? (() => {
            const emptyState = wikiReaderEmptyState(t, pages.length > 0);
            // Vue .wiki-reader-empty: browse icon in a 64px circle, 14px title,
            // 13px description, all in placeholder grays.
            return <div className="wk-wiki-reader-empty wk-wiki-39">
              <span className="wk-wiki-empty-icon wk-wiki-35" aria-hidden="true">
                <WikiGlyph kind="browse" size={48} />
              </span>
              {emptyState.title ? <p className="wk-wiki-40">{emptyState.title}</p> : null}
              {emptyState.description ? <p className="wk-wiki-41">{emptyState.description}</p> : null}
            </div>;
          })() : null}
          {indexView ? (
            <article className="wk-wiki-reader wk-wiki-42" aria-label={t("wikiBrowser.indexTitle")}>
              {/* Vue .wiki-reader-header: 26px/1.3 near-black title + a small
                  light-outline t-tag (bg #f3f3f3, 1px #dcdcdc, radius 3). */}
              <div className="wk-header wk-wiki-43">
                <div className="wk-wiki-44">
                  <h2 className="wiki-reader-title wk-wiki-45">{t("wikiBrowser.indexTitle")}</h2>
                  <p className="wiki-reader-meta wk-wiki-46">{t("wikiBrowser.indexOverviewTag")}</p>
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
            <article className="wk-wiki-reader wk-wiki-42" aria-label={selected.title}>
              <div className="wk-header wk-wiki-43">
                <div className="wk-wiki-44">
                  <h2 className="wiki-reader-title wk-wiki-45">{selected.title}</h2>
                  <p className="wk-muted wk-wiki-47">{selected.summary || "—"}</p>
                </div>
                <div className="wk-list-actions wk-wiki-48">
                  {canContribute ? <TButton type="button" onClick={() => setEditing(true)}>
                    {t("wikiBrowser.editBtn")}
                  </TButton> : null}
                  <TButton type="button" onClick={() => void openHistory()}>
                    {t("wikiBrowser.historyBtn")}
                  </TButton>
                  {canContribute ? <TButton
                    type="button"
                    loading={deleteBusy}
                    onClick={() => void deleteWikiPage()}
                  >
                    {t("wikiBrowser.deletePageBtn")}
                  </TButton> : null}
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
                    className="wk-wiki-reader-content wiki-reader-body wk-wiki-9"
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
            className="wk-wiki-editor wk-wiki-49"
            // The inline editor serves page EDIT mode only (Vue opens it from
            // the 编辑 action); creation lives in the 新建 Wiki 页面 dialog and
            // the index/reader views never show this form.
            style={{ display: editing ? undefined : "none" }}
            onSubmit={save}
          >
            <label>
              {t("wikiBrowser.newPageTitleLabel")}{" "}
              <TdInput
                value={title}
                onChange={(value) => setTitle(String(value))}
              />
            </label>
            <label>
              {t("wikiBrowser.newPageSlugLabel")}{" "}
              <TdInput
                value={slug}
                onChange={(value) => setSlug(String(value))}
                disabled
              />
            </label>
            <label>
              {t("wikiBrowser.editSummaryPlaceholder")}{" "}
              <TdInput
                value={summary}
                onChange={(value) => setSummary(String(value))}
              />
            </label>
            <label>
              {t("wikiBrowser.newPageContentLabel")}{" "}
              <TdTextarea
                value={content}
                onChange={(value) => setContent(String(value))}
                rows={14}
              />
            </label>
            <div className="wk-list-actions wk-wiki-48">
              <span className="wk-wiki-50">{t("wikiBrowser.version", { ver: version })}</span>
              <TButton type="submit">
                {t("wikiBrowser.editSave")}
              </TButton>
              <TButton type="button" onClick={cancelEdit}>
                {t("common.cancel")}
              </TButton>
              <TButton type="button" onClick={() => void reloadSelected()}>
                {t("wikiBrowser.editConflictReload")}
              </TButton>
              <TButton type="button" onClick={() => void openHistory()}>
                {t("wikiBrowser.historyBtn")}
              </TButton>
            </div>
            {saveState?.status === "conflict" ? (
              <div className="wk-wiki-conflict wk-wiki-51">
                <Status tone="warning">{saveState.message}</Status>
                <TButton type="button" onClick={() => void overwriteConflict()}>
                  {t("wikiBrowser.editConflictOverwrite")}
                </TButton>
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
          className="wk-pagination wk-wiki-52"
          aria-label={t("wikiBrowser.page.title")}
        >
          <TButton
            type="button"
            disabled={!pager.hasPrevious}
            onClick={() => setPage((value) => value - 1)}
          >
            {t("wikiBrowser.page.previous")}
          </TButton>
          <span>
            {t("wikiBrowser.page.page", {
              page: pager.page,
              total: pager.total,
            })}
          </span>
          <TButton
            type="button"
            disabled={!pager.hasNext}
            onClick={() => setPage((value) => value + 1)}
          >
            {t("wikiBrowser.page.next")}
          </TButton>
        </nav>
      ) : null}
      {settingsOpen ? (
        <Dialog
          open
          title={t('knowledgeBase.settings')}
          closeLabel={t('common.close')}
          onClose={() => setSettingsOpen(false)}
          className="wk-wiki-53"
        >
          <KnowledgeSettingsPage client={client} knowledgeBaseId={knowledgeBaseId} role={canManage ? 'admin' : 'viewer'} />
        </Dialog>
      ) : null}
      {/* Vue create page dialog (WikiBrowser.vue L733-765): a t-dialog
          「新建 Wiki 页面」 with 标题 / Slug + hint / 页面类型 / 正文（可选）and a
          取消 outline + 确认 primary footer; confirming posts and closes. */}
      {canContribute ? (
        <Dialog
          open={createOpen}
          title={t("wikiBrowser.newPageTitle")}
          closeLabel={t("common.close")}
          onClose={cancelCreate}
          className="wk-wiki-54"
        >
          <form className="wk-wiki-create-form wk-wiki-49" onSubmit={submitCreate}>
            <label>
              {t("wikiBrowser.newPageTitleLabel")}{" "}
              <TdInput
                value={createForm.title}
                onChange={(value) => onTitleInputForCreate(String(value))}
                placeholder={t("wikiBrowser.newPageTitlePlaceholder")}
              />
            </label>
            <label>
              {t("wikiBrowser.newPageSlugLabel")}{" "}
              <TdInput
                value={createForm.slug}
                onChange={(value) => {
                  setCreateForm((current) => ({ ...current, slug: String(value) }));
                  setCreateSlugTouched(true);
                }}
                placeholder={t("wikiBrowser.newPageSlugPlaceholder")}
              />
              <span className="wk-wiki-55">{t("wikiBrowser.newPageSlugHint")}</span>
            </label>
            <label>
              {t("wikiBrowser.newPageTypeLabel")}{" "}
              <select
                className="wk-wiki-56"
                value={createForm.pageType}
                onChange={(event) => setCreateForm((current) => ({ ...current, pageType: event.target.value }))}
              >
                <option value="concept">{t("wikiBrowser.filterConcept")}</option>
                <option value="entity">{t("wikiBrowser.filterEntity")}</option>
                <option value="synthesis">{t("wikiBrowser.filterSynthesis")}</option>
                <option value="comparison">{t("wikiBrowser.filterComparison")}</option>
              </select>
            </label>
            <label>
              {t("wikiBrowser.newPageContentLabel")}{" "}
              <TdTextarea
                value={createForm.content}
                onChange={(value) => setCreateForm((current) => ({ ...current, content: String(value) }))}
                rows={6}
                placeholder={t("wikiBrowser.editContentPlaceholder")}
              />
            </label>
            <div className="wk-list-actions wk-wiki-57">
              {/* Vue create dialog footer: 取消 outline + 确认 primary with a
                  loading confirm (creatingPage). */}
              <TButton type="button" onClick={cancelCreate}>
                {t("common.cancel")}
              </TButton>
              <TButton type="submit" loading={createBusy}>
                {t("common.confirm")}
              </TButton>
            </div>
            {createError ? <Status tone="error">{createError}</Status> : null}
          </form>
        </Dialog>
      ) : null}
      {previewSrc ? <WikiImagePreview src={previewSrc} onClose={() => setPreviewSrc(null)} /> : null}
      {historyOpen && selected ? (
        <Card className="wk-wiki-history wk-wiki-58">
          <div className="wk-header wk-wiki-43">
            <div>
              <h2>{t("wikiBrowser.historyTitle", { title: selected.title })}</h2>
              <p className="wk-muted wk-wiki-59">{t("wikiBrowser.revisionCurrentHint")}</p>
            </div>
            <TButton type="button" onClick={() => setHistoryOpen(false)}>
              {t("common.close")}
            </TButton>
          </div>
          {historyError ? <Status tone="error">{historyError}</Status> : null}
          {historyLoading && !revision ? (
            <Status>{t("wikiBrowser.loading")}</Status>
          ) : null}
          {!historyLoading && revisions.length === 0 ? (
            <Status>{t("wikiBrowser.revisionEmpty")}</Status>
          ) : null}
          <div className="wk-wiki-history-layout wk-wiki-60">
            <nav aria-label={t('wikiBrowser.historyBtn')}>
              <ul className="wk-list wk-wiki-61">
                {revisions.map((item) => (
                  <li key={item.id} className="wk-wiki-62">
                    <button
                      className="wk-wiki-63"
                      type="button"
                      onClick={() => void chooseRevision(item)}
                    >
                      {t("wikiBrowser.version", { ver: item.version })}
                    </button>
                    <span className="wk-wiki-64">{item.edit_source ?? "user"}</span>
                  </li>
                ))}
              </ul>
            </nav>
            <div>
              {revision ? (
                <>
                  <div className="wk-list-actions wk-wiki-48">
                    <strong>
                      v{revision.version} → v{selected.version}
                    </strong>
                    {canContribute ? <TButton
                      type="button"
                      onClick={() => void revertRevision()}
                      loading={reverting}
                    >
                      {t("wikiBrowser.revertBtn")}
                    </TButton> : null}
                  </div>
                  {revisionDiff.length === 0 ? (
                    <Status>{t("wikiBrowser.revisionDiffEmpty")}</Status>
                  ) : (
                    <div className="wk-diff wk-wiki-65">
                      {revisionDiff.map((section) => (
                        <section key={section.field}>
                          <h3>{section.field}</h3>
                          <pre>
                            {section.lines.map((line, index) => (
                              <span
                                key={`${section.field}-${index}`}
                                className={`wk-diff-${line.type} wk-wiki-86 ${line.type === "add" ? "wk-wiki-87" : line.type === "del" ? "wk-wiki-88" : ""}`}
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
      </div>
      {/* Vue DocContent 宿主（关闭态 0 高度，参与 knowledge-layout gap 布局）。 */}
      <div className="doc_content" />
    </div>
  );
}
