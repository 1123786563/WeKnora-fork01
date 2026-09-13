import { useEffect, useMemo, useState } from "react";
import type {
  WikiPage as WikiPageModel,
  WikiPageRevision,
  WikiFolderNode,
  WikiIndexResponse,
  WeKnoraClient,
} from "@weknora/api-client";
import { diffWikiRevision } from "@weknora/domain/wiki/diff";
import { Button, Card, Status } from "@weknora/ui";
import { saveWikiPage, type WikiSaveState } from "./editor.ts";
import { createTranslator, useAppLocale } from "../i18n.ts";
import { pagerState } from "../pagination.ts";

const WIKI_PAGE_SIZE = 50;

export function WikiPage({
  client,
  knowledgeBaseId,
  initialSlug,
  canContribute = true,
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
    <section className="wk-wiki-index" aria-label={t("wikiBrowser.indexTitle")}>
      {indexError ? <Status tone="error">{indexError}</Status> : null}
      {!indexError && indexLoading && !indexView.groups.length ? <Status>{t("wikiBrowser.loading")}</Status> : !indexError && indexView.groups.length === 0 ? <Status>{t("wikiBrowser.indexEmpty")}</Status> : !indexError ? indexView.groups.map((group) => <section key={group.type}><h3>{group.type}</h3><ul className="wk-list">{group.items.map((item) => <li key={item.slug}><button className="wk-document-link" type="button" onClick={() => void client.wiki.get(knowledgeBaseId, item.slug).then(choose)}>{item.title}</button><small>{item.summary}</small></li>)}</ul></section>) : null}
      {indexNextCursor ? <Button type="button" disabled={indexLoading} onClick={() => void loadMoreIndex()}>{indexLoading ? t("wikiBrowser.loading") : t("wikiBrowser.loadMoreShort")}</Button> : null}
    </section>
  ) : (
    <>
      <div className="wk-wiki-directory-toolbar" role="toolbar" aria-label={t("wikiBrowser.viewModeToggle")}>
        <Button type="button" onClick={() => switchViewMode("tree")} aria-pressed={viewMode === "tree"}>{t("wikiBrowser.viewTree")}</Button>
        <Button type="button" onClick={() => switchViewMode("list")} aria-pressed={viewMode === "list"}>{t("wikiBrowser.viewList")}</Button>
        <Button type="button" onClick={() => void openIndex()}>{t("wikiBrowser.indexTitle")}</Button>
        {canContribute ? <Button type="button" disabled={folderBusy} onClick={() => void createFolder()}>{t("wikiBrowser.folderActions")}</Button> : null}
        {folderTrail.length > 0 ? <Button type="button" onClick={backFolder}>{t("wikiBrowser.backToOverview")}</Button> : null}
      </div>
      {viewMode === "tree" && folders.length > 0 ? <ul className="wk-list wk-wiki-folder-list">{folders.map((folder) => <li key={folder.id}><Button type="button" onClick={() => openFolder(folder)}>{folder.name} ({folder.page_count})</Button>{canContribute ? <span className="wk-list-actions"><Button type="button" disabled={folderBusy} onClick={() => void renameFolder(folder)}>{t("wikiBrowser.renameFolder")}</Button><Button type="button" disabled={folderBusy} onClick={() => void deleteFolder(folder)}>{t("wikiBrowser.deleteFolder")}</Button></span> : null}</li>)}</ul> : null}
    </>
  );

  return (
    <main className="wk-page wk-wiki-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p>
          <h1>{t("wikiBrowser.page.title")}</h1>
          <p className="wk-muted">{t("wikiBrowser.page.subtitle")}</p>
        </div>
        <Button type="button" onClick={newPage}>
          {t("wikiBrowser.page.new")}
        </Button>
      </header>
      <Card>
        <div className="wk-wiki-layout">
          <aside className="wk-wiki-sidebar">
            <div className="wk-wiki-sidebar-header">
              <label className="wk-wiki-search" role="search">
                <span className="wk-sr-only">
                  {t("wikiBrowser.page.search")}
                </span>
                <span aria-hidden="true">⌕</span>
                <input
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder={t("wikiBrowser.page.searchPlaceholder")}
                />
              </label>
            </div>
            <nav className="wk-wiki-page-list" aria-label="Wiki pages">
              {directory}
              {!indexView ? pages.map((page) => (
                <button
                  className={`wk-wiki-page-item${selected?.id === page.id ? " is-active" : ""}`}
                  key={page.id}
                  type="button"
                  onClick={() => choose(page)}
                >
                  <span className="wk-wiki-page-item-title">{page.title}</span>
                  <span className="wk-wiki-page-item-summary">
                    {page.summary || "—"}
                  </span>
                  <span className="wk-wiki-page-item-meta">
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
                <div className="wk-wiki-empty">
                  <span className="wk-wiki-empty-icon" aria-hidden="true">
                    ▧
                  </span>
                  <strong>{t("wikiBrowser.emptyTitle")}</strong>
                  <span>{t("wikiBrowser.emptyDesc")}</span>
                </div>
              ) : null}
            </nav>
          </aside>
          {selected && !editing ? (
            <article className="wk-wiki-reader" aria-label={selected.title}>
              <div className="wk-header">
                <div>
                  <h2>{selected.title}</h2>
                  <p className="wk-muted">{selected.summary || "—"}</p>
                </div>
                <div className="wk-list-actions">
                  <Button type="button" onClick={() => setEditing(true)}>
                    Edit
                  </Button>
                  <Button type="button" onClick={() => void openHistory()}>
                    History
                  </Button>
                </div>
              </div>
              <pre className="wk-wiki-reader-content">{selected.content}</pre>
            </article>
          ) : null}
          <form
            className="wk-wiki-editor"
            style={{ display: !selected || editing ? undefined : "none" }}
            onSubmit={save}
          >
            <label>
              Title{" "}
              <input
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                required
              />
            </label>
            <label>
              Slug{" "}
              <input
                value={slug}
                onChange={(event) => setSlug(event.target.value)}
                required
                disabled={Boolean(selected)}
              />
            </label>
            <label>
              Summary{" "}
              <input
                value={summary}
                onChange={(event) => setSummary(event.target.value)}
              />
            </label>
            <label>
              Content{" "}
              <textarea
                value={content}
                onChange={(event) => setContent(event.target.value)}
                rows={14}
                required
              />
            </label>
            <div className="wk-list-actions">
              <span>{selected ? `Version ${version}` : "New page"}</span>
              <Button type="submit">
                {selected ? "Save version" : "Create page"}
              </Button>
              {selected ? (
                <>
                  <Button type="button" onClick={() => void reloadSelected()}>
                    Reload latest
                  </Button>
                  <Button type="button" onClick={() => void openHistory()}>
                    History
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
              <Status tone="success">
                Saved as version {saveState.page.version}.
              </Status>
            ) : null}
          </form>
        </div>
        {pager.total > WIKI_PAGE_SIZE ? (
          <nav
            className="wk-pagination"
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
        <Card className="wk-wiki-history">
          <div className="wk-header">
            <div>
              <h2>Revision history</h2>
              <p className="wk-muted">
                Current page is version {selected.version}; historical snapshots
                are immutable.
              </p>
            </div>
            <Button type="button" onClick={() => setHistoryOpen(false)}>
              Close
            </Button>
          </div>
          {historyError ? <Status tone="error">{historyError}</Status> : null}
          {historyLoading && !revision ? (
            <Status>Loading revisions…</Status>
          ) : null}
          {!historyLoading && revisions.length === 0 ? (
            <Status>No historical revisions.</Status>
          ) : null}
          <div className="wk-wiki-history-layout">
            <nav aria-label="Wiki revisions">
              <ul className="wk-list">
                {revisions.map((item) => (
                  <li key={item.id}>
                    <button
                      className="wk-document-link"
                      type="button"
                      onClick={() => void chooseRevision(item)}
                    >
                      Version {item.version}
                    </button>
                    <span>{item.edit_source ?? "user"}</span>
                  </li>
                ))}
              </ul>
            </nav>
            <div>
              {revision ? (
                <>
                  <div className="wk-list-actions">
                    <strong>
                      v{revision.version} → v{selected.version}
                    </strong>
                    <Button
                      type="button"
                      onClick={() => void revertRevision()}
                      loading={reverting}
                    >
                      Revert to v{revision.version}
                    </Button>
                  </div>
                  {revisionDiff.length === 0 ? (
                    <Status>No changes.</Status>
                  ) : (
                    <div className="wk-diff">
                      {revisionDiff.map((section) => (
                        <section key={section.field}>
                          <h3>{section.field}</h3>
                          <pre>
                            {section.lines.map((line, index) => (
                              <span
                                key={`${section.field}-${index}`}
                                className={`wk-diff-${line.type}`}
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
                <Status>Select a revision to inspect its diff.</Status>
              )}
            </div>
          </div>
        </Card>
      ) : null}
    </main>
  );
}
