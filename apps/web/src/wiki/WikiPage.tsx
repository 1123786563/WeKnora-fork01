import { useEffect, useMemo, useRef, useState } from "react";
import type {
  WikiPage as WikiPageModel,
  WikiPageRevision,
  WikiFolderNode,
  WikiIndexResponse,
  WeKnoraClient,
} from "@weknora/api-client";
import { diffWikiRevision } from "@weknora/domain/wiki/diff";
import { Button, Card, Input, Status, Textarea } from "@weknora/ui";
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
import { pagerState } from "../pagination.ts";
import { wikiEditPermission } from "./edit-permission.ts";
import type { KBSurfaceKB, KBSurfaceMe } from "../knowledge/permissions.ts";

const WIKI_PAGE_SIZE = 50;

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
  const [pageTotal, setPageTotal] = useState(0);
  const [viewMode, setViewMode] = useState<"tree" | "list">("tree");
  const [folderId, setFolderId] = useState("");
  const [folderPath, setFolderPath] = useState("");
  const [folderTrail, setFolderTrail] = useState<Array<{ id: string; name: string; path: string }>>([]);
  const [folders, setFolders] = useState<WikiFolderNode[]>([]);
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
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.auth.me().catch(() => null),
      // Vue canEdit consults the org shared-knowledge-bases list as the
      // authoritative share-grant signal; the probe mirrors those inputs.
      client.identity.organizations.knowledgeBaseShares.listShared().catch(() => null),
    ] as const).then(([kb, me, sharedRows]) => {
      if (!active || !me) return;
      setCanContribute(
        wikiEditPermission(kb as KBSurfaceKB, me as KBSurfaceMe, sharedRows),
      );
    }).catch(() => {
      // Keep the caller's role gate when the optional permission probe fails.
    });
    return () => { active = false; };
  }, [client, knowledgeBaseId, canContributeProp]);

  async function loadPages() {
    setState({ status: "loading" });
    try {
      // Must-fix #3: server-backed pagination replaces the 50-entry hard cap.
      const response = await client.wiki.list(knowledgeBaseId, {
        page,
        page_size: WIKI_PAGE_SIZE,
        keyword: keyword || undefined,
        ...(viewMode === "tree" && folderPath ? { category_path: folderPath } : {}),
      });
      setPages(response.pages);
      setPageTotal(response.total ?? response.pages.length);
      const requested = initialSlug?.trim();
      const requestedPage = requested
        ? response.pages.find((page) => page.slug === requested)
        : undefined;
      if (requestedPage) choose(requestedPage);
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
  }, [client, knowledgeBaseId, keyword, initialSlug, page, folderPath, viewMode]);
  useEffect(() => {
    setPage(1);
  }, [keyword, folderPath, viewMode]);

  useEffect(() => {
    if (viewMode !== "tree") return;
    if (typeof client.wiki.folders !== "function") return;
    void client.wiki.folders(knowledgeBaseId, folderId).then((result) => setFolders(result.folders)).catch(() => setFolders([]));
  }, [client, knowledgeBaseId, folderId, viewMode]);

  async function reloadFolders() {
    const result = await client.wiki.folders(knowledgeBaseId, folderId);
    setFolders(result.folders);
  }

  async function openIndex() {
    if (typeof client.wiki.index !== "function") return;
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

  async function createFolder() {
    const name = window.prompt(t("wikiBrowser.folderNamePlaceholder"))?.trim();
    if (!name || folderBusy) return;
    setFolderBusy(true);
    try { await client.wiki.createFolder(knowledgeBaseId, folderId, name); await reloadFolders(); }
    catch (error) { setState({ status: "error", message: error instanceof Error ? error.message : t("wikiBrowser.createFolderFailed") }); }
    finally { setFolderBusy(false); }
  }

  async function renameFolder(folder: WikiFolderNode) {
    const name = window.prompt(t("wikiBrowser.folderNamePlaceholder"), folder.name)?.trim();
    if (!name || name === folder.name || folderBusy) return;
    setFolderBusy(true);
    try { await client.wiki.updateFolder(knowledgeBaseId, folder.id, { name }); await reloadFolders(); }
    catch (error) { setState({ status: "error", message: error instanceof Error ? error.message : t("wikiBrowser.renameFolderFailed") }); }
    finally { setFolderBusy(false); }
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
  const directory = (
    <>
      <div className="wk-wiki-directory-toolbar flex flex-wrap items-center gap-[0.35rem] pb-2" role="toolbar" aria-label={t("wikiBrowser.viewModeToggle")}>
        {/* Vue renders these as icon-only buttons with tooltips — the labels
            live in aria-label/title, not on the button face. */}
        <Button type="button" aria-pressed={viewMode === "tree"} aria-label={t("wikiBrowser.viewTree")} title={t("wikiBrowser.viewTree")} onClick={() => switchViewMode("tree")}>☰</Button>
        <Button type="button" aria-pressed={viewMode === "list"} aria-label={t("wikiBrowser.viewList")} title={t("wikiBrowser.viewList")} onClick={() => switchViewMode("list")}>≡</Button>
        <Button type="button" aria-label={t("wikiBrowser.indexTitle")} title={t("wikiBrowser.indexTitle")} onClick={() => void openIndex()}>{t("wikiBrowser.indexTitle")}</Button>
        {canContribute ? <Button type="button" disabled={folderBusy} aria-label={t("wikiBrowser.folderActions")} title={t("wikiBrowser.folderActions")} onClick={() => void createFolder()}>＋</Button> : null}
        {folderTrail.length > 0 ? <Button type="button" onClick={backFolder}>{t("wikiBrowser.backToOverview")}</Button> : null}
      </div>
      {viewMode === "tree" && folders.length > 0 ? <ul className="wk-list wk-wiki-folder-list m-0 mb-2 list-none p-0 pb-2">{folders.map((folder) => <li key={folder.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><Button type="button" onClick={() => openFolder(folder)}>{folder.name} ({folder.page_count})</Button>{canContribute ? <span className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="button" disabled={folderBusy} onClick={() => void renameFolder(folder)}>{t("wikiBrowser.renameFolder")}</Button><Button type="button" disabled={folderBusy} onClick={() => void deleteFolder(folder)}>{t("wikiBrowser.deleteFolder")}</Button></span> : null}</li>)}</ul> : null}
    </>
  );

  return (
    <main className="wk-page wk-wiki-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12">
      <header className="wk-header mb-6 flex items-start justify-between gap-4">
        <div>
          <p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('common.knowledgeBases')}</p>
          <h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{t("wikiBrowser.page.title")}</h1>
          <p className="wk-muted text-muted">{t("wikiBrowser.page.subtitle")}</p>
        </div>
        {canContribute ? <Button type="button" onClick={newPage}>
          {t("wikiBrowser.page.new")}
        </Button> : null}
      </header>
      <Card>
        <div className="wk-wiki-layout grid grid-cols-[minmax(220px,320px)_1fr] gap-5 max-[720px]:grid-cols-1">
          <aside className="wk-wiki-sidebar flex min-w-0 flex-col border-r border-[#e7e7e7]">
            <div className="wk-wiki-sidebar-header pr-2.5 pb-2">
              <form className="wk-wiki-search flex items-center gap-[0.45rem] rounded-md border border-[#e7e7e7] bg-[#f3f3f3] px-[0.6rem] py-[0.45rem] text-[rgba(0,0,0,0.4)]" role="search" onSubmit={(event) => { event.preventDefault(); submitSearch(); }}>
                <span className="wk-sr-only absolute h-px w-px m-[-1px] overflow-hidden [clip:rect(0_0_0_0)]">
                  {t("wikiBrowser.page.search")}
                </span>
                <span aria-hidden="true">⌕</span>
                <Input
                  value={searchDraft}
                  onChange={(event) => setSearchDraft(event.target.value)}
                  placeholder={t("wikiBrowser.searchPlaceholder")}
                />
              </form>
            </div>
            <nav className="wk-wiki-page-list flex max-h-[620px] flex-col gap-0.5 overflow-y-auto pr-2.5 pb-3" aria-label={t('wikiBrowser.pageActions')}>
              {directory}
              {listPages.map((page) => (
                <button
                  className={`wk-wiki-page-item group/wiki-item grid min-h-[98px] cursor-pointer gap-0.5 rounded-md border-0 bg-transparent px-2.5 py-2 text-left transition-colors duration-150 hover:bg-[#f0f3f8] ${selected?.id === page.id ? "bg-[#eef4ef]" : ""}`}
                  key={page.id}
                  type="button"
                  onClick={() => choose(page)}
                >
                  <span className="wk-wiki-page-item-title truncate text-sm leading-5 text-[#202020]">{page.title}</span>
                  <span className="wk-wiki-page-item-summary line-clamp-2 text-xs leading-[1.5] text-[rgba(0,0,0,0.6)]">
                    {page.summary || "—"}
                  </span>
                  <span className="wk-wiki-page-item-meta text-[11px] text-[rgba(0,0,0,0.4)]">
                    v{page.version}
                  </span>
                </button>
              ))}
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
                  <h2>{t("wikiBrowser.indexTitle")}</h2>
                  <p className="wk-muted text-muted">{t("wikiBrowser.indexOverviewTag")}</p>
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
            style={{ display: !selected || editing ? undefined : "none" }}
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
      </Card>
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
