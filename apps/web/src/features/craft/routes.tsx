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
import type { CraftAccessMember, CraftBudgetExtensionAction, CraftGrantableAccessRole, CraftInputDecisionAction, CraftSubmitRunInput, WeKnoraClient } from '@weknora/api-client';
import { createCraftApi, createServerSentEventParser, craftDownloadPath } from '@weknora/api-client';
import { submitDraftWithAttachments } from '@weknora/core/craft/command-bridge';
import type { CraftCapabilitiesView, CraftInputView, CraftSessionKind, CraftSessionSummaryView, CraftVersionView, CraftWorkspaceView } from '@weknora/contracts';
import type { ScopeController } from '@weknora/domain/scope';
import { createCraftWorkbenchController, type CraftEventFrame, type CraftEventTransport, type CraftSyncError } from '@weknora/core/craft/controller';
import { authorizationHeader, type LegacyPlatformSession } from '../../platform/legacy-session.ts';
import { CraftHome, capabilitiesFromView, type CraftAttachmentDraft, type CraftHomeCreateInput } from '@weknora/views/craft/home';
import { CraftLibrary } from '@weknora/views/craft/library';
import { CraftTemplates } from '@weknora/views/craft/templates';
import { CraftWorkbench, createCraftWorkbenchFeatures, type CraftInteractionActionInput } from '@weknora/views/craft/workbench';
import { CraftAccess } from '@weknora/views/craft/access';
import { CraftEditRequestPanel } from '@weknora/views/craft/workbench-edit';
import { CraftInputDecisionPanel } from '@weknora/views/craft/files';
import { CraftInputExpandPanel } from '@weknora/views/craft/input-expand';
import { CRAFT_USAGE_STRINGS_ZH, CraftBudgetPauseNotice } from '@weknora/views/craft/usage';
import { createCraftMessageLog, downloadFileName, type CraftLocale } from '@weknora/views/craft/presentation';
import { createSessionCraftInteractionClient, CraftInteractionPanel } from '@weknora/views/craft/interaction';
import { CraftSharePanel, projectShareView, type CraftShareView } from '@weknora/views/craft/share';
import { CraftExportConsentPanel, projectExportConsentView, type CraftExportConsentView } from '@weknora/views/craft/export';

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
function createTeeTransport(
  log: ReturnType<typeof createCraftMessageLog>,
  inner: CraftEventTransport,
  activeDelegation: { current: { runId: string; taskId: string } | null },
): CraftEventTransport {
  return {
    subscribe: (input) =>
      inner.subscribe({
        ...input,
        onEvent: (frame: CraftEventFrame) => {
          log.resetForRun(input.runId);
          log.ingest(frame);
          // R06: remember the active delegation so the stop button can address it
          // (the craft frame payload carries delegation_id — the stop task id).
          try {
            const parsed = JSON.parse(frame.data) as { delegation_id?: string };
            if (typeof parsed.delegation_id === 'string' && parsed.delegation_id !== '') {
              activeDelegation.current = { runId: input.runId, taskId: parsed.delegation_id };
            }
          } catch { /* non-JSON frames pass through untouched */ }
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

// T20 central assembly (#128/#133): the consent panels ride the aside slot,
// bound to the workbench's SELECTED version — the exact immutable version
// the owner decides on. Both load through the api-client seams and
// re-project the raw body fail-closed; the server stays the sole consent
// authority (the panels only render, they never infer authority).
type CraftRoutesApi = ReturnType<typeof createCraftApi>;
type CraftShareConsentState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; view: CraftShareView };

function CraftShareConsentSection(props: {
  locale: CraftLocale;
  role: 'owner' | 'collaborator' | 'viewer';
  sessionId: string;
  versionId: string;
  api: CraftRoutesApi;
  scope: ScopeController;
}) {
  const [state, setState] = useState<CraftShareConsentState>({ status: 'loading' });
  const load = useCallback(async () => {
    setState({ status: 'loading' });
    try {
      const view = projectShareView(await props.api.shareView(props.sessionId, props.versionId, props.scope.current().signal));
      setState(view === null ? { status: 'error' } : { status: 'ready', view });
    } catch {
      setState({ status: 'error' });
    }
  }, [props.api, props.sessionId, props.versionId, props.scope]);
  useEffect(() => { void load(); }, [load]);
  if (state.status === 'loading') return null;
  if (state.status === 'error') {
    return <p role="alert">Share consent unavailable. Restricted-source sharing stays closed until it loads.</p>;
  }
  // An unrestricted version carries no share decision to make or show.
  if (!state.view.restricted) return null;
  const apply = (raw: unknown) => {
    const next = projectShareView(raw);
    setState(next === null ? { status: 'error' } : { status: 'ready', view: next });
  };
  return <CraftSharePanel
    locale={props.locale}
    role={props.role}
    view={state.view}
    onDecide={(decision) => props.api.decideShare(props.sessionId, props.versionId, decision, state.view.evidenceDigest, props.scope.current().signal).then(apply)}
    onRevoke={() => props.api.revokeShare(props.sessionId, props.versionId, props.scope.current().signal).then(apply)}
  />;
}

type CraftExportConsentState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; view: CraftExportConsentView };

function CraftExportConsentSection(props: {
  locale: CraftLocale;
  role: 'owner' | 'collaborator' | 'viewer';
  sessionId: string;
  versionId: string;
  api: CraftRoutesApi;
  scope: ScopeController;
}) {
  const [state, setState] = useState<CraftExportConsentState>({ status: 'loading' });
  const load = useCallback(async () => {
    setState({ status: 'loading' });
    try {
      const view = projectExportConsentView(await props.api.exportConsent(props.sessionId, props.versionId, props.scope.current().signal));
      setState(view === null ? { status: 'error' } : { status: 'ready', view });
    } catch {
      setState({ status: 'error' });
    }
  }, [props.api, props.sessionId, props.versionId, props.scope]);
  useEffect(() => { void load(); }, [load]);
  if (state.status === 'loading') return null;
  if (state.status === 'error') {
    return <p role="alert">Export consent unavailable. Restricted derived files stay withheld until it loads.</p>;
  }
  // A "none" state means no derived file rides restricted origins — there
  // is no export decision to make or show for this version.
  if (state.view.state === 'none') return null;
  const apply = (raw: unknown) => {
    const next = projectExportConsentView(raw);
    setState(next === null ? { status: 'error' } : { status: 'ready', view: next });
  };
  return <CraftExportConsentPanel
    locale={props.locale}
    role={props.role}
    view={state.view}
    onDecide={(decision) => props.api.decideExportConsent(props.sessionId, props.versionId, decision, state.view.manifestDigest, props.scope.current().signal).then(apply)}
  />;
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
  // R06: the run + delegation the stop button addresses. Every craft frame's
  // payload refreshes it (the tee parses each frame for delegation_id).
  const activeDelegationRef = useRef<{ runId: string; taskId: string } | null>(null);
  const transport = useMemo(
    () => createTeeTransport(messageLog, createCraftSseTransport(authedFetch), activeDelegationRef),
    [messageLog, authedFetch],
  );
  const [syncError, setSyncError] = useState<string | null>(null);
  // C03: transient reconnect states (stream ended, gap, cursor expiry,
  // offline) show the syncing banner and CLEAR it once sync recovers;
  // syncError stays reserved for permanent failures.
  const [syncNotice, setSyncNotice] = useState<string | null>(null);
  // R06 stop entrance: 'stopping' is the honest phase while an accepted stop
  // polls delegationStatus toward terminal; the workbench shows the busy label.
  const [stopPhase, setStopPhase] = useState<'idle' | 'stopping'>('idle');
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


  // Identity for the owner write gate (the terminal entry is owner-scoped).
  const [meId, setMeId] = useState<string | null>(null);
  const [identifiedSession, setIdentifiedSession] = useState<LegacyPlatformSession | null>(null);
  useEffect(() => {
    if (session.credential.kind === 'anonymous') {
      setMeId(null);
      setIdentifiedSession(session);
      return;
    }
    let cancelled = false;
    setIdentifiedSession(null);
    void client
      .request({ method: 'GET', path: '/api/v1/auth/me' })
      .then((value: unknown) => {
        if (cancelled) return;
        const row = value as { data?: { user?: { id?: unknown } } };
        setMeId(typeof row?.data?.user?.id === 'string' ? row.data.user.id : null);
        setIdentifiedSession(session);
      })
      .catch(() => {
        if (!cancelled) {
          setMeId(null);
          setIdentifiedSession(session);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [client, session]);
  const currentMeId = identifiedSession === session ? meId : null;

  const [accessState, setAccessState] = useState<{
    sessionId: string;
    status: 'loading' | 'ready' | 'error';
    members: CraftAccessMember[];
    error: string | null;
  }>({ sessionId: '', status: 'loading', members: [], error: null });

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
  // C01/T10 (#127) sources panel opener — switched (T20/#139) to the TESTED
  // HTTP seam: ONE fetch through GET /craft/runs/:run_id/sources/:citation_id/
  // open passes BOTH doors (the current Task grant AND the caller's own
  // knowledge ACL) and returns only the opaque durable ref. The former
  // client-side craftkb:// parsing walked the viewer's knowledge ACL but
  // never the TaskOpenSource gate. Nothing is cached or pre-signed; a denial
  // (or a revoked share) fails HERE, at click time, and no URL/href is ever
  // rendered — the panel's only currency stays the durable ref.
  const openKnowledgeSource = useCallback(
    async (citationId: string, _ref: string, runId: string | null): Promise<string | null> => {
      if (sessionId === null || runId === null || runId === '') return '来源不可打开（当前无运行上下文）';
      try {
        const wire = await craftApi.openSource(sessionId, runId, citationId, scopeController.current().signal);
        const ref = typeof (wire as { ref?: unknown } | null)?.ref === 'string' ? (wire as { ref: string }).ref : '';
        return ref !== '' ? '来源已通过当前权限校验（引用可见）' : '服务器响应无效';
      } catch {
        // Denials carry no reason by design (T10); every click re-runs both
        // doors, so a later grant succeeds on the next click.
        return '引用 ' + citationId + ' 的来源已不可见（无权限或已删除）';
      }
    },
    [craftApi, sessionId, scopeController],
  );
  const [workbenchInfo, setWorkbenchInfo] = useState<{ title: string; kind: string; ownerId: string; snapshotVersionId: string | null; updatedAt: string; resumed: boolean; activeRun: { id: string; status: string; waitReason: string } | null } | null>(null);
  const [versions, setVersions] = useState<{ status: 'loading' | 'ready' | 'error'; items: CraftVersionView[] }>({ status: 'loading', items: [] });
  const [inputDecisionState, setInputDecisionState] = useState<{ sessionId: string; inputs: CraftInputView[] }>({ sessionId: '', inputs: [] });
  // T20/T02 (#121): inputs the member associated with THIS Task in this
  // workbench session — the archive-expansion entrance projects exactly
  // these (the server re-validates association on every expand; a reconnect
  // simply re-associates or re-expands through the same guarded endpoint).
  const [associatedInputs, setAssociatedInputs] = useState<{ sessionId: string; inputs: CraftInputView[] }>({ sessionId: '', inputs: [] });
  const pendingRequestIdRef = useRef<string | null>(null);
  const pendingSubmitRef = useRef<{
    sessionId: string;
    attachmentIds: string[];
    body: CraftSubmitRunInput;
  } | null>(null);
  const pendingInputDecisions = useRef(new Map<string, {
    sessionId: string;
    resolve(ref: string): void;
    reject(error: Error): void;
  }>());

  const activeSessionId = useRef<string | null>(null);
  activeSessionId.current = sessionId;

  useEffect(() => {
    setInputDecisionState({ sessionId: sessionId ?? '', inputs: [] });
    setAssociatedInputs({ sessionId: sessionId ?? '', inputs: [] });
    pendingRequestIdRef.current = null;
    pendingSubmitRef.current = null;
    return () => {
      pendingRequestIdRef.current = null;
      pendingSubmitRef.current = null;
      if (sessionId === null) return;
      for (const [key, pending] of pendingInputDecisions.current) {
        if (pending.sessionId === sessionId) {
          pendingInputDecisions.current.delete(key);
          pending.reject(new Error('Task changed before the input decision was acknowledged.'));
        }
      }
    };
  }, [sessionId]);

  const refreshAccess = useCallback(async (targetSessionId: string) => {
    const members = await craftApi.accessMembers(targetSessionId, scopeController.current().signal);
    if (activeSessionId.current === targetSessionId) {
      setAccessState({ sessionId: targetSessionId, status: 'ready', members, error: null });
    }
  }, [craftApi, scopeController]);

  useEffect(() => {
    if (sessionId === null) {
      setAccessState({ sessionId: '', status: 'loading', members: [], error: null });
      return;
    }
    let cancelled = false;
    setAccessState({ sessionId, status: 'loading', members: [], error: null });
    void craftApi.accessMembers(sessionId, scopeController.current().signal)
      .then((members) => {
        if (!cancelled) setAccessState({ sessionId, status: 'ready', members, error: null });
      })
      .catch((error: unknown) => {
        if (!cancelled) setAccessState({ sessionId, status: 'error', members: [], error: error instanceof Error ? error.message : String(error) });
      });
    return () => { cancelled = true; };
  }, [craftApi, sessionId, scopeController]);

  const grantAccess = useCallback(async (userId: string, grantRole: CraftGrantableAccessRole) => {
    if (sessionId === null) throw new Error('No active Craft Task');
    await craftApi.grantAccess(sessionId, userId, grantRole, scopeController.current().signal);
    try {
      await refreshAccess(sessionId);
    } catch (error) {
      if (activeSessionId.current === sessionId) {
        setAccessState({ sessionId, status: 'error', members: [], error: error instanceof Error ? error.message : String(error) });
      }
    }
  }, [craftApi, sessionId, scopeController, refreshAccess]);

  const revokeAccess = useCallback(async (userId: string) => {
    if (sessionId === null) throw new Error('No active Craft Task');
    await craftApi.revokeAccess(sessionId, userId, scopeController.current().signal);
    try {
      await refreshAccess(sessionId);
    } catch (error) {
      if (activeSessionId.current === sessionId) {
        setAccessState({ sessionId, status: 'error', members: [], error: error instanceof Error ? error.message : String(error) });
      }
    }
  }, [craftApi, sessionId, scopeController, refreshAccess]);

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
          activeRun: toActiveRun(view),
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

  const canWrite = workbenchInfo !== null && currentMeId !== null && workbenchInfo.ownerId === currentMeId;

  // The CURRENT member's access row, derived once per (session, access
  // view, identity): both the task-access entry and the edit-request entry
  // consume this same source — one lookup, one matching key, no drift.
  // undefined means the role is not (yet) confirmed for this view.
  const currentMember = useMemo(() => {
    if (currentMeId === null || accessState.sessionId !== sessionId || accessState.status !== 'ready') return undefined;
    return accessState.members.find((member) => member.user_id === currentMeId);
  }, [currentMeId, accessState, sessionId]);

  // T20 (#139) budget-pause panel mount: when the workspace's active Run is
  // durably parked on the budget wait, the pause view is fetched ONCE per
  // run identity. Extension actors (owner/billing admin) get the server's
  // extension-action projection; everyone else is refused server-side and the
  // assembly keeps the contact-owner copy without any figures (the panel
  // renders no limit/used, so nothing is fabricated).
  const activeRun = workbenchInfo?.activeRun ?? null;
  // The pause view's numbers exist ONLY when the server answered: a denied
  // fetch (the T19 seam restricts the view to extension actors) degrades to
  // the contact-owner notice WITHOUT fabricating limit/used — unknown stays
  // unknown, never folded into zeros.
  const [budgetPauseView, setBudgetPauseView] = useState<
    | { runId: string; viewed: true; extensionAction: CraftBudgetExtensionAction | null; limit: number; used: number }
    | { runId: string; viewed: false }
  | null>(null);
  const [budgetExtensionBusy, setBudgetExtensionBusy] = useState(false);
  const budgetExtensionAction = budgetPauseView?.viewed ? budgetPauseView.extensionAction : null;
  const budgetExtensionInFlightRef = useRef(false);
  useEffect(() => {
    setBudgetPauseView(null);
    if (sessionId === null || activeRun === null || activeRun.waitReason !== 'budget_exhausted') return;
    let cancelled = false;
    const runId = activeRun.id;
    void craftApi.budgetPause(sessionId, runId, scopeController.current().signal)
      .then((view) => {
        if (!cancelled) setBudgetPauseView({ runId, viewed: true, extensionAction: view.extensionAction, limit: view.pause.limit, used: view.pause.used });
      })
      .catch(() => {
        if (!cancelled) setBudgetPauseView({ runId, viewed: false });
      });
    return () => {
      cancelled = true;
    };
  }, [sessionId, activeRun, craftApi, scopeController]);
  // refreshWorkbench re-reads the authoritative workspace projection and
  // merges the active-run facts into the mounted info (the same merge the
  // initial load performs through the same toActiveRun projection — the
  // initial load replaces the WHOLE info shape, this refresh merges only
  // the active-run facts). Both run-state-changing callers reuse it: the
  // admitted edit request (best-effort) and the budget extension.
  // toActiveRun is the ONE workspace active-run projection every merge site
  // uses — a field change is made exactly once.
  const toActiveRun = (view: CraftWorkspaceView): { id: string; status: string; waitReason: string } | null =>
    view.active_run === null
      ? null
      : { id: view.active_run.run_id, status: view.active_run.status, waitReason: view.active_run.wait_reason };
  const refreshWorkbench = useCallback(async (targetSessionId: string): Promise<void> => {
    const view = await craftApi.get(targetSessionId, scopeController.current().signal);
    if (activeSessionId.current !== targetSessionId) return;
    setWorkbenchInfo((prev) => prev === null ? prev : {
      ...prev,
      resumed: view.active_run !== null,
      activeRun: toActiveRun(view),
    });
  }, [craftApi, scopeController]);

  // The server owns the pending action tuple. Reuse this projected action on
  // retries; a remount fetches the same pending intent from the server again.
  // A ref also closes the rapid double-click gap before React commits busy.
  const requestBudgetExtension = useCallback(
    async (runId: string, action: CraftBudgetExtensionAction): Promise<void> => {
      if (sessionId === null || budgetExtensionInFlightRef.current) return;
      budgetExtensionInFlightRef.current = true;
      setBudgetExtensionBusy(true);
      try {
        await craftApi.extendBudget(sessionId, runId, action, scopeController.current().signal);
        // The Run left the pause durably: reload the authoritative projection
        // (workspace view drives the panel away) and the controller state.
        try {
          await refreshWorkbench(sessionId);
        } catch {
          // The controller reload below still reflects the resumed state; the
          // panel stays until the next workspace refresh.
        }
        await controller.load(sessionId);
      } finally {
        budgetExtensionInFlightRef.current = false;
        setBudgetExtensionBusy(false);
      }
    },
    [sessionId, craftApi, scopeController, controller, refreshWorkbench],
  );

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
  const decisionKey = (targetSessionId: string, ref: string) => `${targetSessionId}\u0000${ref}`;
  const waitForInputDecision = useCallback((targetSessionId: string, input: CraftInputView): Promise<string> => {
    if (activeSessionId.current !== targetSessionId) return Promise.reject(new Error('Task changed before the input decision was requested.'));
    const key = decisionKey(targetSessionId, input.ref);
    return new Promise<string>((resolve, reject) => {
      pendingInputDecisions.current.set(key, { sessionId: targetSessionId, resolve, reject });
      setInputDecisionState((prior) => ({
        sessionId: targetSessionId,
        inputs: prior.sessionId === targetSessionId && !prior.inputs.some((pending) => pending.ref === input.ref)
          ? [...prior.inputs, input]
          : prior.sessionId === targetSessionId ? prior.inputs : [input],
      }));
    });
  }, []);

  const decideInput = useCallback(async (ref: string, action: CraftInputDecisionAction): Promise<void> => {
    if (sessionId === null || activeSessionId.current !== sessionId) throw new Error('Task changed before the input decision was recorded.');
    const key = decisionKey(sessionId, ref);
    const pending = pendingInputDecisions.current.get(key);
    if (pending === undefined || pending.sessionId !== sessionId) throw new Error('This input decision is no longer active.');
    const acknowledgement = await craftApi.decideInput(sessionId, ref, action, scopeController.current().signal);
    if (activeSessionId.current !== sessionId || pendingInputDecisions.current.get(key) !== pending) {
      throw new Error('Task changed before the input decision was acknowledged.');
    }
    pendingInputDecisions.current.delete(key);
    if (acknowledgement.action === 'continue') pending.resolve(acknowledgement.ref);
    else pending.reject(new Error('Run cancelled because an unrecognized input was not continued.'));
  }, [craftApi, scopeController, sessionId]);

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
          if (activeSessionId.current !== sessionId) throw new Error('Task changed while the attachment was being accepted.');
          // T20/T02: track the accepted input so the archive-expansion
          // entrance can offer the guarded server endpoint for it.
          setAssociatedInputs((prior) => ({
            sessionId,
            inputs: prior.sessionId === sessionId && !prior.inputs.some((row) => row.ref === input.ref)
              ? [...prior.inputs, input]
              : prior.sessionId === sessionId ? prior.inputs : [input],
          }));
          if (input.recognition === null || !input.recognition.accepted) {
            throw new Error('Craft input acceptance was not confirmed by the server.');
          }
          if (!input.recognition.understood) {
            const decidedRef = await waitForInputDecision(sessionId, input);
            if (activeSessionId.current !== sessionId) throw new Error('Task changed before the Run could be submitted.');
            return decidedRef;
          }
          return input.ref;
        }
        if (status === 'failed') throw new Error(`attachment processing failed: ${attachment.name}`);
        await delay(1000);
      }
      throw new Error(`attachment processing timed out: ${attachment.name}`);
    },
    [sessionId, authedFetch, craftApi, scopeController, waitForInputDecision],
  );

  const setAttachmentState = useCallback((id: string, state: HomeAttachment['state']) => {
    setAttachments((prev) => prev.map((item) => (item.id === id ? { ...item, state } : item)));
  }, []);

  const enrichedSend = useCallback(
    async (prompt: string): Promise<void> => {
      const current = attachments;
      if (sessionId === null) throw new Error('no active craft session');

      const pendingSubmit = pendingSubmitRef.current;
      if (pendingSubmit !== null) {
        const attachmentsMatch = pendingSubmit.attachmentIds.length === current.length
          && pendingSubmit.attachmentIds.every((id, index) => id === current[index]?.id);
        if (pendingSubmit.sessionId !== sessionId) {
          throw new Error('A previous Run outcome belongs to another Task. Start a new send in this Task.');
        }
        if (pendingRequestIdRef.current !== pendingSubmit.body.request_id) {
          throw new Error('The previous Run outcome is unresolved. Refresh this Task before starting another send.');
        }
        if (prompt !== pendingSubmit.body.prompt || !attachmentsMatch) {
          throw new Error('A previous Run outcome is unresolved. Retry the original prompt and attachments before changing them.');
        }

        // The server may already have admitted this Run. Replay the immutable
        // wire body directly so retries cannot upload a second input ref.
        const replay: CraftSubmitRunInput = {
          request_id: pendingSubmit.body.request_id,
          prompt: pendingSubmit.body.prompt,
          input_refs: [...(pendingSubmit.body.input_refs ?? [])],
          knowledge_scope: pendingSubmit.body.knowledge_scope,
          base_version_id: pendingSubmit.body.base_version_id,
        };
        await craftApi.submit(sessionId, replay, scopeController.current().signal);
        pendingRequestIdRef.current = null;
        pendingSubmitRef.current = null;
        setInputDecisionState({ sessionId, inputs: [] });
        setAttachments([]);
        setCreationScope(null);
        setCreationPrompt(null);
        await controller.load(sessionId);
        return;
      }

      setInputDecisionState({ sessionId, inputs: [] });
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
            submitDraft: (command, signal) => {
              if (activeSessionId.current !== sessionId) throw new Error('Task changed before the Run could be submitted.');
              const body: CraftSubmitRunInput = Object.freeze({
                request_id: command.request_id,
                prompt: command.prompt,
                input_refs: Object.freeze([...command.input_refs]) as unknown as string[],
                knowledge_scope: command.knowledge_scope,
                base_version_id: command.base_version_id,
              });
              pendingSubmitRef.current = {
                sessionId,
                attachmentIds: Object.freeze(current.map((attachment) => attachment.id)) as unknown as string[],
                body,
              };
              return craftApi.submit(sessionId, {
                ...body,
                input_refs: [...(body.input_refs ?? [])],
              }, signal ?? scopeController.current().signal);
            },
          },
        );
      } catch (error) {
        // uploads or the submission failed: the intent KEEPS its key so the
        // user's retry replays idempotently
        throw error;
      }
      pendingRequestIdRef.current = null;
      pendingSubmitRef.current = null;
      setInputDecisionState({ sessionId, inputs: [] });
      setAttachments([]);
      setCreationScope(null);
      setCreationPrompt(null);
      await controller.load(sessionId);
    },
    [attachments, sessionId, uploadAndAssociate, craftApi, creationScope, scopeController, controller, setAttachmentState, setInputDecisionState],
  );

  const accessFeatures = useMemo(() => createCraftWorkbenchFeatures([
    {
      name: 'input-decision',
      // Keep the gate visible while the mobile workbench is on its
      // conversation tab; placing it in the aside could deadlock a send.
      slot: 'header',
      render: () => sessionId === null ? null : <CraftInputDecisionPanel
        key={sessionId}
        locale={locale}
        inputs={inputDecisionState.sessionId === sessionId ? inputDecisionState.inputs : []}
        onDecide={decideInput}
      />,
    },
    {
      // T20/T02 (#121): the archive-expansion entrance rides the guarded
      // POST /craft/inputs/expand endpoint; members the server published
      // join the tracked inputs (nested archives re-enter the same gate).
      name: 'input-expand',
      slot: 'header',
      render: () => sessionId === null ? null : <CraftInputExpandPanel
        key={sessionId + '-expand'}
        locale={locale}
        inputs={associatedInputs.sessionId === sessionId ? associatedInputs.inputs : []}
        onExpand={(ref) => craftApi.expandInput(sessionId, ref, scopeController.current().signal).then((members) => {
          setAssociatedInputs((prior) => prior.sessionId === sessionId
            ? { sessionId, inputs: [...prior.inputs, ...members.filter((member) => !prior.inputs.some((row) => row.ref === member.ref))] }
            : { sessionId, inputs: [...members] });
          return members;
        })}
      />,
    },
    {
      name: 'task-access',
      slot: 'aside',
      render: () => {
        if (sessionId === null) return null;
        if (accessState.sessionId !== sessionId || accessState.status === 'loading') {
          return <p role="status">Loading Task access…</p>;
        }
        if (accessState.status === 'error') {
          return <p role="alert">Task access unavailable: {accessState.error ?? 'Could not load access information.'}</p>;
        }
        if (currentMember === undefined) {
          return <p role="status">Your Task role could not be confirmed. Access controls are unavailable.</p>;
        }
        return <CraftAccess
          role={currentMember.role}
          members={accessState.members}
          onGrant={grantAccess}
          onRevoke={revokeAccess}
        />;
      },
    },
    {
      // T20/T09 (#135): the collaborator serialized-edit panel. canWrite is
      // the SERVER-derived current role (owner or collaborator — the access
      // view's member row, never a client guess); runActive mirrors the
      // workspace's authoritative active-run projection; each request rides
      // a fresh request_id through the raw-envelope submit seam and the
      // panel holds the single fail-closed projection (writer lease
      // conflict, unknown lease and the recorded initiator included).
      name: 'edit-request',
      slot: 'aside',
      render: () => {
        if (sessionId === null) return null;
        // An unconfirmed role is NOT a read-only verdict: while the access
        // view loads (or failed to load) the panel stays hidden instead of
        // asserting "read-only member" about a possibly-writing member —
        // the task-access entry keeps its dedicated loading/error wording.
        if (currentMember === undefined) return null;
        // owner OR collaborator — the TaskWrite grant. Deliberately NOT the
        // component-scope canWrite (owner-only upload authority): this
        // panel's seam is the server-derived serialized-edit grant.
        const canRequestEdit = currentMember.role === 'owner' || currentMember.role === 'collaborator';
        return <CraftEditRequestPanel
          key={sessionId + '-edit'}
          locale={locale}
          canWrite={canRequestEdit}
          runActive={activeRun !== null}
          onRequestEdit={(prompt) => craftApi.submitEdit(sessionId, {
            request_id: crypto.randomUUID(), prompt,
          }, scopeController.current().signal).then((raw) => {
            // The admitted run occupies the Task's single slot: refresh the
            // authoritative projection (the serialization notice and the
            // budget-pause wiring see it) and RELOAD the controller so the
            // workbench subscribes to the new run's event stream. Only the
            // budget extension performs this full pair on its own path;
            // enrichedSend relies on controller.load alone (its event
            // stream refreshes the run projection) (best-effort: the panel
            // already answered from the submit envelope).
            void refreshWorkbench(sessionId).catch(() => {});
            void controller.load(sessionId).catch(() => {});
            return raw;
          })}
        />;
      },
    },
    {
      // T20/T11 (#128): the restricted-source share consent panel. Like the
      // edit-request panel, the role is the SERVER-derived current member
      // row (never a client guess) and an unconfirmed role hides the panel
      // instead of asserting read-only about a possibly-writing member.
      name: 'share-consent',
      slot: 'aside',
      render: (context) => {
        if (sessionId === null || currentMember === undefined) return null;
        if (context.selectedVersionId === null) return null;
        return <CraftShareConsentSection
          key={sessionId + ':' + context.selectedVersionId + '-share'}
          locale={locale}
          role={currentMember.role}
          sessionId={sessionId}
          versionId={context.selectedVersionId}
          api={craftApi}
          scope={scopeController}
        />;
      },
    },
    {
      // T20/T13 (#133): the restricted-derived export consent panel, bound
      // to the same selected version the download buttons act on.
      name: 'export-consent',
      slot: 'aside',
      render: (context) => {
        if (sessionId === null || currentMember === undefined) return null;
        if (context.selectedVersionId === null) return null;
        return <CraftExportConsentSection
          key={sessionId + ':' + context.selectedVersionId + '-export'}
          locale={locale}
          role={currentMember.role}
          sessionId={sessionId}
          versionId={context.selectedVersionId}
          api={craftApi}
          scope={scopeController}
        />;
      },
    },
  ]), [sessionId, locale, inputDecisionState, associatedInputs, decideInput, craftApi, scopeController, accessState, currentMeId, grantAccess, revokeAccess, activeRun, refreshWorkbench, controller]);

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
          features={accessFeatures}
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
          stopPhase={stopPhase}
          onStopRun={async () => {
            const active = activeDelegationRef.current;
            if (active === null || sessionId === null) return;
            setStopPhase('stopping');
            try {
              const result = await craftApi.stop(sessionId, active.runId, active.taskId, scopeController.current().signal);
              // Poll the read-only status until the phase leaves "stopping".
              for (let i = 0; i < 60 && result.phase === 'stopping'; i++) {
                await new Promise((resolve) => setTimeout(resolve, 2000));
                const status = await craftApi.delegationStatus(sessionId, active.runId, active.taskId, scopeController.current().signal);
                if (status.phase !== 'stopping') break;
              }
            } catch (error) {
              // The stop request itself failed: surface it through the same
              // banner the controller's permanent failures use.
              setSyncError(error instanceof Error ? error.message : String(error));
            } finally {
              setStopPhase('idle');
              activeDelegationRef.current = null;
              // The controller's own snapshot reload reflects the terminal
              // state — the same entry the existing error/cancel paths use.
              await controller.load(sessionId);
            }
          }}
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
        {budgetPauseView !== null ? (
          budgetPauseView.viewed ? (
            <CraftBudgetPauseNotice
              pause={{ run_id: budgetPauseView.runId, reason: 'exhausted', limit: budgetPauseView.limit, used: budgetPauseView.used }}
              canExtend={budgetExtensionAction !== null}
              onRequestExtension={budgetExtensionAction !== null && !budgetExtensionBusy ? (runId) => {
                void requestBudgetExtension(runId, budgetExtensionAction).catch((error: unknown) => {
                  setSyncError(error instanceof Error ? error.message : String(error));
                });
              } : undefined}
            />
          ) : (
            // Denied pause view: the same member-visible copy the panel
            // uses, WITHOUT the numeric pause object — nothing fabricated.
            <aside role="status" data-testid="craft-budget-pause">
              <strong>{CRAFT_USAGE_STRINGS_ZH.pauseTitle}</strong>
              <p>{CRAFT_USAGE_STRINGS_ZH.pauseContactOwner}</p>
            </aside>
          )
        ) : null}
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
