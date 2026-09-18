// W05 craft route assembly (apps/web side).
//
// This module MOUNTS the craft views through the existing web assembly — the
// legacy platform session (login), the shared scope controller (space), and
// the shared WeKnora client — without introducing a second router: craft
// routes are two paths (/craft and /craft/:sessionId) resolved from
// window.location with pushState/popstate only.
//
// It owns every platform concern the view components must not touch:
//   * auth headers (legacy bearer/embed credential) for raw SSE + downloads;
//   * the R05-style SSE transport over the EXISTING run-events endpoint,
//     teed into the workbench message log (same frames the W04 controller
//     consumes — the views never see a second projection);
//   * the create flow: create session → upload attachments through the
//     EXISTING session attachment entrance → associate via POST /craft/inputs
//     → submit the first run (enriched sends; plain sends go through the W04
//     controller.submit directly);
//   * preview ticket issuing (W02), fixed-versionId downloads through the
//     wildcard files route (W03), the authorized sandbox terminal ticket,
//     and the /auth/me identity used for the owner write gate.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createCraftApi, createServerSentEventParser, craftDownloadPath } from '@weknora/api-client';
import { submitDraftWithAttachments } from '@weknora/core/craft/command-bridge';
import type { CraftCapabilitiesView, CraftSessionKind, CraftSessionSummaryView, CraftVersionView } from '@weknora/contracts';
import type { ScopeController } from '@weknora/domain/scope';
import { createCraftWorkbenchController, type CraftEventFrame, type CraftEventTransport, type CraftSyncError } from '@weknora/core/craft/controller';
import { authorizationHeader, type LegacyPlatformSession } from '../../platform/legacy-session.ts';
import { CraftHome, capabilitiesFromView, type CraftAttachmentDraft, type CraftHomeCreateInput } from '@weknora/views/craft/home';
import { CraftLibrary } from '@weknora/views/craft/library';
import { CraftTemplates } from '@weknora/views/craft/templates';
import { CraftWorkbench, type CraftInteractionActionInput } from '@weknora/views/craft/workbench';
import { createCraftMessageLog, downloadFileName, type CraftLocale } from '@weknora/views/craft/presentation';
import { createSessionCraftInteractionClient, CraftInteractionPanel } from '@weknora/views/craft/interaction';

type CraftRoute =
  | { name: 'home' }
  | { name: 'library' }
  | { name: 'templates' }
  | { name: 'workbench'; sessionId: string };

function parseCraftRoute(pathname: string): CraftRoute | null {
  if (pathname === '/craft' || pathname === '/craft/') return { name: 'home' };
  if (pathname === '/craft/library' || pathname === '/craft/library/') return { name: 'library' };
  if (pathname === '/craft/templates' || pathname === '/craft/templates/') return { name: 'templates' };
  const match = pathname.match(/^\/craft\/([^/]+)\/?$/);
  if (match !== null) return { name: 'workbench', sessionId: decodeURIComponent(match[1]) };
  return null;
}

/** Auth-header-injecting fetch for raw streams/downloads the JSON transport cannot carry. */
function createAuthedFetch(session: LegacyPlatformSession, baseUrl: string): typeof fetch {
  return (input, init) => {
    const headers = new Headers(init?.headers);
    const authorization = authorizationHeader(session.credential);
    if (authorization !== undefined) headers.set('Authorization', authorization);
    if (session.tenantId !== null) headers.set('X-Tenant-ID', session.tenantId);
    return fetch(typeof input === 'string' && input.startsWith('/') ? baseUrl + input : input, { ...init, headers });
  };
}

/** SSE transport over the EXISTING run-events endpoint (R05 pattern, shared parser). */
function createCraftSseTransport(authedFetch: typeof fetch): CraftEventTransport {
  return {
    subscribe: async ({ sessionId, runId, after, signal, onEvent }) => {
      const url = `/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/events?after=${after}`;
      const response = await authedFetch(url, { headers: { accept: 'text/event-stream' }, signal });
      if (!response.ok || response.body === null) throw new Error(`craft stream HTTP ${response.status}`);
      const parser = createServerSentEventParser(onEvent);
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        parser.push(decoder.decode(value, { stream: true }));
        if (signal.aborted) break;
      }
      parser.finish();
    },
  };
}

/** Every backend frame feeds BOTH the W04 controller and the message log. */
function createTeeTransport(log: ReturnType<typeof createCraftMessageLog>, inner: CraftEventTransport): CraftEventTransport {
  return {
    subscribe: (input) =>
      inner.subscribe({
        ...input,
        onEvent: (frame: CraftEventFrame) => {
          log.resetForRun(input.runId);
          log.ingest(frame);
          input.onEvent(frame);
        },
      }),
  };
}

async function sha256Hex(file: File): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
  return Array.from(new Uint8Array(digest)).map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

interface HomeAttachment extends CraftAttachmentDraft {
  file: File;
}

export interface CraftRoutesProps {
  client: WeKnoraClient;
  scopeController: ScopeController;
  session: LegacyPlatformSession;
  apiBaseUrl: string;
}

export function CraftRoutes(props: CraftRoutesProps) {
  const { client, scopeController, session, apiBaseUrl } = props;

  const [route, setRoute] = useState<CraftRoute>(() => parseCraftRoute(window.location.pathname) ?? { name: 'home' });
  useEffect(() => {
    const onPopState = () => setRoute(parseCraftRoute(window.location.pathname) ?? { name: 'home' });
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);
  const navigate = useCallback((path: string) => {
    window.history.pushState(null, '', path);
    setRoute(parseCraftRoute(path) ?? { name: 'home' });
  }, []);

  const [locale, setLocale] = useState<CraftLocale>('zh');

  const craftApi = useMemo(() => createCraftApi(client.request), [client]);
  const authedFetch = useMemo(() => createAuthedFetch(session, apiBaseUrl), [session, apiBaseUrl]);

  // The controller + teed message log live for the whole craft session.
  const messageLog = useMemo(() => createCraftMessageLog(), []);
  const transport = useMemo(() => createTeeTransport(messageLog, createCraftSseTransport(authedFetch)), [messageLog, authedFetch]);
  const [syncError, setSyncError] = useState<string | null>(null);
  // C03: transient reconnect states (stream ended, gap, cursor expiry,
  // offline) show the syncing banner and CLEAR it once sync recovers;
  // syncError stays reserved for permanent failures.
  const [syncNotice, setSyncNotice] = useState<string | null>(null);
  const controller = useMemo(
    () =>
      createCraftWorkbenchController({
        api: craftApi,
        scope: scopeController,
        events: transport,
        onError: (error: CraftSyncError) => setSyncError(`${error.code}`),
        onSync: (phase, error) => setSyncNotice(phase === 'syncing' ? (error?.code ?? 'RECONNECTING') : null),
      }),
    [craftApi, scopeController, transport],
  );
  useEffect(() => () => controller.dispose(), [controller]);

  // C02: the interaction decide surface rides the authed fetch (list +
  // decide through the existing session permission chain), bound per
  // workbench session; the panel mounts below the workbench.
  const interactions = useMemo(
    () => createSessionCraftInteractionClient(authedFetch, route.name === 'workbench' ? route.sessionId : '', apiBaseUrl),
    [authedFetch, route, apiBaseUrl],
  );

  // C01 sources panel: resolve one durable craftkb:// ref through the
  // EXISTING knowledge permission chain on every click (never cached, never
  // pre-signed): the documents list of the owning library answers 403/404
  // through the same ACL the build went through, and the resolved row
  // becomes the panel notice. A revoked share fails HERE, at click time.
  const openKnowledgeSource = useCallback(
    async (citationId: string, ref: string): Promise<string | null> => {
      const match = /^craftkb:\/\/kb\/([^/]+)\/knowledge\/([^/]+)\//.exec(ref);
      if (match === null) return '无法解析来源引用 ' + ref;
      const kbId = match[1] ?? '';
      const knowledgeId = match[2] ?? '';
      const page = await client.knowledgeBases.documents.list(kbId, {});
      const row = page.data.find((item) => item.id === knowledgeId);
      if (row === undefined) return '引用 ' + citationId + ' 的来源已不可见（无权限或已删除）';
      return '已解析来源：' + (row.title ?? row.file_name ?? knowledgeId) + '（在所属知识库查看全文）';
    },
    [client],
  );


  // Identity for the owner write gate (the terminal entry is owner-scoped).
  const [meId, setMeId] = useState<string | null>(null);
  useEffect(() => {
    if (session.credential.kind === 'anonymous') {
      setMeId(null);
      return;
    }
    let cancelled = false;
    void client
      .request({ method: 'GET', path: '/api/v1/auth/me' })
      .then((value: unknown) => {
        if (cancelled) return;
        const row = value as { data?: { user?: { id?: unknown } } };
        setMeId(typeof row?.data?.user?.id === 'string' ? row.data.user.id : null);
      })
      .catch(() => {
        if (!cancelled) setMeId(null);
      });
    return () => {
      cancelled = true;
    };
  }, [client, session]);

  // --- Home data -------------------------------------------------------------
  const [homeList, setHomeList] = useState<{ status: 'loading' | 'error' | 'ready'; error: string | null; items: CraftSessionSummaryView[]; nextCursor: string | null; capabilities: CraftCapabilitiesView | null }>({
    status: 'loading',
    error: null,
    items: [],
    nextCursor: null,
    capabilities: null,
  });
  const loadHomeList = useCallback(
    async (cursor?: string) => {
      setHomeList((prev) => ({ status: 'loading', error: null, items: cursor === undefined ? prev.items : [], nextCursor: prev.nextCursor, capabilities: prev.capabilities }));
      try {
        const page = await craftApi.list(cursor === undefined || cursor === '' ? {} : { cursor }, scopeController.current().signal);
        setHomeList({ status: 'ready', error: null, items: page.data, nextCursor: page.next_cursor, capabilities: page.capabilities ?? null });
      } catch (error) {
        setHomeList((prev) => ({ status: 'error', error: error instanceof Error ? error.message : String(error), items: [], nextCursor: null, capabilities: prev.capabilities }));
      }
    },
    [craftApi, scopeController],
  );

  const [knowledgeOptions, setKnowledgeOptions] = useState<{ id: string; name: string }[]>([]);
  useEffect(() => {
    if (session.credential.kind === 'anonymous') return;
    let cancelled = false;
    void client
      .knowledgeBases
      .list({})
      .then((bases) => {
        if (cancelled) return;
        setKnowledgeOptions(bases.map((base) => ({ id: base.id, name: base.name })));
      })
      .catch(() => {
        if (!cancelled) setKnowledgeOptions([]);
      });
    return () => {
      cancelled = true;
    };
  }, [client, session]);

  // Attachments picked on Home travel with the creation into the workbench.
  const [attachments, setAttachments] = useState<HomeAttachment[]>([]);
  // P03: a template fill lands here, then the home creator consumes it once.
  const [templateDraft, setTemplateDraft] = useState<{ goal: string; kind: CraftSessionKind } | null>(null);
  const [creationPrompt, setCreationPrompt] = useState<string | null>(null);
  const [creationScope, setCreationScope] = useState<string | null>(null);
  const [createBusy, setCreateBusy] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const handleCreate = async (input: CraftHomeCreateInput): Promise<void> => {
    setCreateBusy(true);
    setCreateError(null);
    try {
      const created = await craftApi.create(
        { request_id: crypto.randomUUID(), title: input.title, kind: input.kind },
        scopeController.current().signal,
      );
      setCreationPrompt(input.title);
      setCreationScope(input.knowledgeScope);
      navigate('/craft/' + encodeURIComponent(created.session_id));
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : String(error));
    } finally {
      setCreateBusy(false);
    }
  };

  // --- Workbench data ----------------------------------------------------------
  const sessionId = route.name === 'workbench' ? route.sessionId : null;
  const [workbenchInfo, setWorkbenchInfo] = useState<{ title: string; kind: string; ownerId: string; snapshotVersionId: string | null; updatedAt: string; resumed: boolean } | null>(null);
  const [versions, setVersions] = useState<{ status: 'loading' | 'ready' | 'error'; items: CraftVersionView[] }>({ status: 'loading', items: [] });

  const loadVersions = useCallback(async () => {
    if (sessionId === null) return;
    try {
      const page = await craftApi.versions(sessionId, scopeController.current().signal);
      setVersions({ status: 'ready', items: page.data });
    } catch {
      setVersions({ status: 'error', items: [] });
    }
  }, [craftApi, sessionId, scopeController]);

  useEffect(() => {
    if (sessionId === null) {
      setWorkbenchInfo(null);
      setVersions({ status: 'loading', items: [] });
      void loadHomeList();
      return;
    }
    let cancelled = false;
    setWorkbenchInfo(null);
    setVersions({ status: 'loading', items: [] });
    setSyncError(null);
    setSyncNotice(null);
    const signal = scopeController.current().signal;
    void (async () => {
      try {
        const view = await craftApi.get(sessionId, signal);
        if (cancelled) return;
        // The workspace view carries no updated_at; the sessions list row has
        // the honest per-session time for the history panel.
        let updatedAt = '';
        try {
          const page = await craftApi.list({}, signal);
          const row = page.data.find((item) => item.session_id === sessionId);
          if (row !== undefined) updatedAt = row.updated_at;
        } catch {
          updatedAt = '';
        }
        if (cancelled) return;
        setWorkbenchInfo({
          title: view.title,
          kind: view.kind,
          ownerId: view.workspace.user_id,
          snapshotVersionId: view.current_version === null ? null : view.current_version.id,
          updatedAt,
          resumed: view.active_run !== null,
        });
        await loadVersions();
        await controller.load(sessionId);
      } catch (error) {
        if (!cancelled) setSyncError(error instanceof Error ? error.message : String(error));
      }
    })();
    return () => {
      cancelled = true;
    };
    // controller.load is invoked deliberately per sessionId mount; controller
    // identity is stable for the routes' lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, craftApi, scopeController, controller, loadVersions, loadHomeList]);

  const canWrite = workbenchInfo !== null && meId !== null && workbenchInfo.ownerId === meId;

  const issuePreview = useCallback(
    async (versionId: string) => craftApi.preview(sessionId ?? '', versionId, scopeController.current().signal),
    [craftApi, sessionId, scopeController],
  );

  // D01 wiring: the kind preview surfaces read immutable version members
  // through the SAME authorized wildcard files route the download uses —
  // report.md/manifest.json for documents, preview.json for spreadsheet and
  // slides, and slides page images as object URLs the <img> may load.
  const fetchVersionFile = useCallback(
    async (versionId: string, path: string): Promise<string> => {
      if (sessionId === null) throw new Error('no active craft session');
      const response = await authedFetch(craftDownloadPath(sessionId, versionId, path));
      if (!response.ok) throw new Error('version file fetch failed: HTTP ' + response.status);
      return await response.text();
    },
    [sessionId, authedFetch],
  );

  const resolveVersionFileUrl = useCallback(
    async (versionId: string, path: string): Promise<string> => {
      if (sessionId === null) throw new Error('no active craft session');
      const response = await authedFetch(craftDownloadPath(sessionId, versionId, path));
      if (!response.ok) throw new Error('version asset fetch failed: HTTP ' + response.status);
      return URL.createObjectURL(await response.blob());
    },
    [sessionId, authedFetch],
  );

  const downloadFile = useCallback(
    async (versionId: string, path: string): Promise<void> => {
      if (sessionId === null) return;
      const url = craftDownloadPath(sessionId, versionId, path);
      const response = await authedFetch(url);
      if (!response.ok) throw new Error(`download failed: HTTP ${response.status}`);
      const blob = await response.blob();
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = downloadFileName(path);
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(objectUrl);
    },
    [sessionId, authedFetch],
  );

  const mintTerminalUrl = useCallback(async (): Promise<string> => {
    if (sessionId === null) throw new Error('no active craft session');
    const response = await authedFetch(`/api/v1/sessions/${encodeURIComponent(sessionId)}/sandbox/terminal-ticket`, { method: 'POST' });
    if (!response.ok) throw new Error(`terminal ticket failed: HTTP ${response.status}`);
    const payload = (await response.json()) as { data?: { ticket?: unknown } };
    const ticket = payload?.data?.ticket;
    if (typeof ticket !== 'string' || ticket === '') throw new Error('terminal ticket missing');
    const base = apiBaseUrl !== '' ? new URL(apiBaseUrl) : new URL(window.location.origin);
    const protocol = base.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${base.host}/api/v1/sessions/${encodeURIComponent(sessionId)}/sandbox/terminal?ticket=${encodeURIComponent(ticket)}`;
  }, [sessionId, authedFetch, apiBaseUrl]);

  // The interaction decide routes do not exist on the backend yet (W04 report
  // §6): decisions surface here and nowhere else — never a silent approval.
  const pendingDecisions = useRef<CraftInteractionActionInput[]>([]);
  // CFT-S01-T008: the live submit intent's idempotency key. Null = no live
  // intent; minted on first send, reused across retries, cleared on success.
  const pendingRequestIdRef = useRef<string | null>(null);
  const handleInteractionAction = useCallback((input: CraftInteractionActionInput) => {
    pendingDecisions.current = [...pendingDecisions.current, input];
    // Visible feedback without pretending the backend accepted it.
    window.console.warn('[craft] interaction decision recorded locally (backend decide route pending):', input.action, input.id);
  }, []);

  const uploadAndAssociate = useCallback(
    async (attachment: HomeAttachment): Promise<string> => {
      if (sessionId === null) throw new Error('no active craft session');
      const digest = await sha256Hex(attachment.file);
      const form = new FormData();
      form.append('file', attachment.file);
      const upload = await authedFetch(`/api/v1/sessions/${encodeURIComponent(sessionId)}/attachments`, { method: 'POST', body: form });
      if (!upload.ok) throw new Error(`attachment upload failed: HTTP ${upload.status}`);
      const uploadBody = (await upload.json()) as { data?: { id?: unknown } };
      const attachmentId = uploadBody?.data?.id;
      if (typeof attachmentId !== 'string' || attachmentId === '') throw new Error('attachment upload returned no id');
      for (let attempt = 0; attempt < 60; attempt += 1) {
        const statusResponse = await authedFetch(`/api/v1/sessions/${encodeURIComponent(sessionId)}/attachments/${encodeURIComponent(attachmentId)}`);
        if (!statusResponse.ok) throw new Error(`attachment status failed: HTTP ${statusResponse.status}`);
        const statusBody = (await statusResponse.json()) as { data?: { status?: unknown } };
        const status = statusBody?.data?.status;
        if (status === 'ready') {
          // The association answer carries the canonical input ref (resource://…)
          // the run submission must reference — the raw attachment id is only
          // the upload's address, not the workspace input ref (W06 finding).
          const input = await craftApi.addInput(sessionId, { resource_ref: attachmentId, expected_sha256: digest }, scopeController.current().signal);
          return input.ref;
        }
        if (status === 'failed') throw new Error(`attachment processing failed: ${attachment.name}`);
        await delay(1000);
      }
      throw new Error(`attachment processing timed out: ${attachment.name}`);
    },
    [sessionId, authedFetch, craftApi, scopeController],
  );

  const setAttachmentState = useCallback((id: string, state: HomeAttachment['state']) => {
    setAttachments((prev) => prev.map((item) => (item.id === id ? { ...item, state } : item)));
  }, []);

  const enrichedSend = useCallback(
    async (prompt: string): Promise<void> => {
      const current = attachments;
      if (sessionId === null) throw new Error('no active craft session');
      // CFT-S01-T008: the submit intent is frozen through the command bridge.
      // The requestId is minted ONCE per intent and survives lost responses —
      // a retry of the same composer draft reuses it, so the server replays
      // the admission instead of admitting twice. It clears only on success.
      if (pendingRequestIdRef.current === null) pendingRequestIdRef.current = crypto.randomUUID();
      try {
        await submitDraftWithAttachments(
          {
            sessionId,
            prompt,
            kind: 'web',
            knowledgeScope: creationScope ?? '',
            baseVersionId: null,
            requestId: pendingRequestIdRef.current,
            expectedWorkspaceRevision: null,
            attachments: current.map((attachment) => ({ name: attachment.name, sha256: '', file: attachment.file })),
            signal: scopeController.current().signal,
          },
          {
            uploadAndAssociate: async (bridgeAttachment) => {
              const attachment = current.find((item) => item.name === bridgeAttachment.name);
              if (attachment === undefined) throw new Error(`unknown attachment: ${bridgeAttachment.name}`);
              setAttachmentState(attachment.id, 'uploading');
              try {
                const ref = await uploadAndAssociate(attachment);
                setAttachmentState(attachment.id, 'ready');
                return ref;
              } catch (error) {
                setAttachmentState(attachment.id, 'error');
                throw error;
              }
            },
            submitDraft: (command, signal) =>
              craftApi.submit(sessionId, command, signal ?? scopeController.current().signal),
          },
        );
      } catch (error) {
        // uploads or the submission failed: the intent KEEPS its key so the
        // user's retry replays idempotently
        throw error;
      }
      pendingRequestIdRef.current = null;
      setAttachments([]);
      setCreationScope(null);
      setCreationPrompt(null);
      await controller.load(sessionId);
    },
    [attachments, sessionId, uploadAndAssociate, craftApi, creationScope, scopeController, controller, setAttachmentState],
  );

  if (route.name === 'home') {
    return (
      <div>
        <LocaleToggle locale={locale} onChange={setLocale} />
        <CraftHome
          locale={locale}
          canCreate={session.credential.kind !== 'anonymous'}
          capabilities={capabilitiesFromView(homeList.capabilities)}
          initial={templateDraft === null ? undefined : { goal: templateDraft.goal, kind: templateDraft.kind }}
          createBusy={createBusy}
          createError={createError}
          listStatus={homeList.status}
          listError={homeList.error}
          recent={homeList.items}
          nextCursor={homeList.nextCursor}
          knowledgeOptions={knowledgeOptions}
          attachments={attachments}
          onCreate={(input) => void handleCreate(input)}
          onPickAttachments={(files) => {
            setAttachments((prev) => [
              ...prev,
              ...files.map((file) => ({ id: crypto.randomUUID(), name: file.name, state: 'pending' as const, file })),
            ]);
          }}
          onRemoveAttachment={(id) => setAttachments((prev) => prev.filter((item) => item.id !== id))}
          onOpen={(id) => navigate('/craft/' + encodeURIComponent(id))}
          onNextPage={(cursor) => void loadHomeList(cursor)}
          onRetryList={() => void loadHomeList()}
        />
      </div>
    );
  }

  if (route.name === 'library') {
    return (
      <div>
        <LocaleToggle locale={locale} onChange={setLocale} />
        <CraftLibrary
          locale={locale}
          sessions={homeList.items}
          status={homeList.status}
          error={homeList.error}
          hasNextPage={homeList.nextCursor !== null && homeList.nextCursor !== ''}
          loadingMore={homeList.status === 'loading' && homeList.items.length > 0}
          onOpen={(id) => navigate('/craft/' + encodeURIComponent(id))}
          onLoadMore={() => void loadHomeList(homeList.nextCursor ?? undefined)}
          onRetry={() => void loadHomeList()}
        />
      </div>
    );
  }

  if (route.name === 'templates') {
    return (
      <div>
        <LocaleToggle locale={locale} onChange={setLocale} />
        <CraftTemplates
          capabilities={capabilitiesFromView(homeList.capabilities)}
          onUse={(template) => {
            // P03: a template ONLY fills the create form — never authorizes,
            // never executes, never pre-selects knowledge.
            setTemplateDraft({ goal: template.goal, kind: template.kind });
            navigate('/craft');
          }}
        />
      </div>
    );
  }

  return (
    <div>
      <LocaleToggle locale={locale} onChange={setLocale} />
      {workbenchInfo === null ? (
        <main className="wk-craft-page">
          <p className="wk-craft-muted" role="status">{syncError === null ? 'Loading…' : syncError}</p>
        </main>
      ) : (
        <div>
        <CraftWorkbench
          locale={locale}
          sessionId={route.sessionId}
          title={workbenchInfo.title}
          kind={workbenchInfo.kind}
          canWrite={canWrite}
          controller={controller}
          messageLog={messageLog}
          initialPrompt={creationPrompt}
          resumedRun={workbenchInfo.resumed}
          pendingAttachments={attachments}
          enrichedPending={attachments.length > 0 || creationScope !== null}
          onPickAttachments={(files) => {
            setAttachments((prev) => [
              ...prev,
              ...files.map((file) => ({ id: crypto.randomUUID(), name: file.name, state: 'pending' as const, file })),
            ]);
          }}
          onRemoveAttachment={(id) => setAttachments((prev) => prev.filter((item) => item.id !== id))}
          onEnrichedSend={enrichedSend}
          versions={versions.items}
          sessionUpdatedAt={workbenchInfo.updatedAt}
          snapshotVersionId={workbenchInfo.snapshotVersionId}
          onRefreshVersions={() => void loadVersions()}
          onIssuePreview={issuePreview}
          onDownload={downloadFile}
          onFetchVersionFile={fetchVersionFile}
          onResolveVersionFileUrl={resolveVersionFileUrl}
          onInteractionAction={handleInteractionAction}
          onOpenSource={openKnowledgeSource}
          onMintTerminalUrl={mintTerminalUrl}
          onBack={() => navigate('/craft')}
          syncError={syncNotice ?? syncError}
        />
        <CraftInteractionPanel
          locale={locale}
          sessionId={route.sessionId}
          client={interactions}
          canDecide={canWrite}
          pollMs={5000}
        />
        </div>
      )}
    </div>
  );
}

function LocaleToggle(props: { locale: CraftLocale; onChange(locale: CraftLocale): void }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'flex-end', maxWidth: 1180, margin: '0 auto', padding: '0.5rem 1.25rem 0' }}>
      <button
        type="button"
        className="wk-craft-tab"
        onClick={() => props.onChange(props.locale === 'zh' ? 'en' : 'zh')}
        lang={props.locale === 'zh' ? 'en' : 'zh'}
      >
        {props.locale === 'zh' ? 'English' : '中文'}
      </button>
    </div>
  );
}