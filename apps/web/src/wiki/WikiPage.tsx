import { useEffect, useMemo, useState } from "react";
import type {
  WikiPage as WikiPageModel,
  WikiPageRevision,
  WikiFolderNode,
  WikiIndexResponse,
  WeKnoraClient,
} from "@weknora/api-client";
import { diffWikiRevision } from "@weknora/domain/wiki/diff";
import { Button, Card, Input, Status, Textarea } from "@weknora/ui";
import { saveWikiPage, type WikiSaveState } from "./editor.ts";
import { createTranslator, useAppLocale } from "../i18n.ts";
import { pagerState } from "../pagination.ts";
import { computeKBPermissions, type KBSurfaceKB, type KBSurfaceMe } from "../knowledge/permissions.ts";

const WIKI_PAGE_SIZE = 50;

export function WikiPage({
  client,
  knowledgeBaseId,
  initialSlug,
  canContribute: canContributeProp = false,
}: {
  client: WeKnoraClient;
  knowledgeBaseId: string;
  initialSlug?: string;
  canContribute?: boolean;
}) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [page, setPage] = useState(1);
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
  const [canContribute, setCanContribute] = useState(canContributeProp);

  useEffect(() => {
    let active = true;
    setCanContribute(canContributeProp);
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.auth.me().catch(() => null),
    ] as const).then(([kb, me]) => {
      if (!active || !me) return;
      setCanContribute(
        computeKBPermissions(kb as KBSurfaceKB, me as KBSurfaceMe).canContribute,
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
      choose(await client.wiki.get(knowledgeBaseId, selected.slug));
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
        `Revert ${selected.title} to version ${revision.version}?`,
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
          : t("wikiBrowser.revisionLoadFailed"),
      );
    } finally {
      setReverting(false);
    }
  }
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
  const directory = indexView ? (
    <section className="wk-wiki-index grid gap-3 pb-2" aria-label={t("wikiBrowser.indexTitle")}>
      {indexError ? <Status tone="error">{indexError}</Status> : null}
      {!indexError && indexLoading && !indexView.groups.length ? <Status>{t("wikiBrowser.loading")}</Status> : !indexError && indexView.groups.length === 0 ? <Status>{t("wikiBrowser.indexEmpty")}</Status> : !indexError ? indexView.groups.map((group) => <section key={group.type}><h3>{group.type}</h3><ul className="wk-list m-0 list-none p-0">{group.items.map((item) => <li key={item.slug} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><button className="border-0 bg-transparent cursor-pointer p-0 text-left text-primary-deep [font:inherit] [font-weight:650]! hover:underline" type="button" onClick={() => void client.wiki.get(knowledgeBaseId, item.slug).then(choose)}>{item.title}</button><small>{item.summary}</small></li>)}</ul></section>) : null}
      {indexNextCursor ? <Button type="button" disabled={indexLoading} onClick={() => void loadMoreIndex()}>{indexLoading ? t("wikiBrowser.loading") : t("wikiBrowser.loadMoreShort")}</Button> : null}
    </section>
  ) : (
    <>
      <div className="wk-wiki-directory-toolbar flex flex-wrap items-center gap-[0.35rem] pb-2" role="toolbar" aria-label={t("wikiBrowser.viewModeToggle")}>
        <Button type="button" onClick={() => switchViewMode("tree")} aria-pressed={viewMode === "tree"}>{t("wikiBrowser.viewTree")}</Button>
        <Button type="button" onClick={() => switchViewMode("list")} aria-pressed={viewMode === "list"}>{t("wikiBrowser.viewList")}</Button>
        <Button type="button" onClick={() => void openIndex()}>{t("wikiBrowser.indexTitle")}</Button>
        {canContribute ? <Button type="button" disabled={folderBusy} onClick={() => void createFolder()}>{t("wikiBrowser.folderActions")}</Button> : null}
        {folderTrail.length > 0 ? <Button type="button" onClick={backFolder}>{t("wikiBrowser.backToOverview")}</Button> : null}
      </div>
      {viewMode === "tree" && folders.length > 0 ? <ul className="wk-list wk-wiki-folder-list m-0 mb-2 list-none p-0 pb-2">{folders.map((folder) => <li key={folder.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><Button type="button" onClick={() => openFolder(folder)}>{folder.name} ({folder.page_count})</Button>{canContribute ? <span className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="button" disabled={folderBusy} onClick={() => void renameFolder(folder)}>{t("wikiBrowser.renameFolder")}</Button><Button type="button" disabled={folderBusy} onClick={() => void deleteFolder(folder)}>{t("wikiBrowser.deleteFolder")}</Button></span> : null}</li>)}</ul> : null}
    </>
  );

  return (
    <main className="wk-page wk-wiki-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12">
      <header className="wk-header mb-6 flex items-start justify-between gap-4">
        <div>
          <p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('common.knowledgeBases')} · {knowledgeBaseId}</p>
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
              <label className="wk-wiki-search flex items-center gap-[0.45rem] rounded-md border border-[#e7e7e7] bg-[#f3f3f3] px-[0.6rem] py-[0.45rem] text-[rgba(0,0,0,0.4)]" role="search">
                <span className="wk-sr-only absolute h-px w-px m-[-1px] overflow-hidden [clip:rect(0_0_0_0)]">
                  {t("wikiBrowser.page.search")}
                </span>
                <span aria-hidden="true">⌕</span>
                <Input
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder={t("wikiBrowser.page.searchPlaceholder")}
                />
              </label>
            </div>
            <nav className="wk-wiki-page-list flex max-h-[620px] flex-col gap-0.5 overflow-y-auto pr-2.5 pb-3" aria-label={t('wikiBrowser.pageActions')}>
              {directory}
              {!indexView ? pages.map((page) => (
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
              )) : null}
              {!indexView && state.status === "loading" ? (
                <Status>Loading Wiki pages…</Status>
              ) : null}
              {!indexView && state.status === "error" ? (
                <Status tone="error">{state.message}</Status>
              ) : null}
              {!indexView && state.status === "success" && pages.length === 0 ? (
                <div className="wk-wiki-empty flex flex-1 flex-col items-center gap-2 px-5 py-[60px] text-center text-[rgba(0,0,0,0.6)]">
                  <span className="wk-wiki-empty-icon text-[36px] leading-none text-[#07c05f]" aria-hidden="true">
                    ▧
                  </span>
                  <strong>{t("wikiBrowser.emptyTitle")}</strong>
                  <span>{t("wikiBrowser.emptyDesc")}</span>
                </div>
              ) : null}
            </nav>
          </aside>
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
                </div>
              </div>
              <pre className="wk-wiki-reader-content m-0 box-border min-h-[22rem] overflow-auto rounded-md border border-[#d8e0eb] bg-[#f8fafc] p-4 font-[inherit] leading-[1.65] whitespace-pre-wrap">{selected.content}</pre>
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
              <Status tone="warning">{saveState.message}</Status>
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
