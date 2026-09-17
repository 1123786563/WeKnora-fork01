import * as React from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { Text, View } from 'react-native';
import { useRoute } from '@react-navigation/native';
import { SessionView } from '@/-session/SessionView';
import { ConversationScreen } from '@/weknora/conversations/ConversationScreen';
import type { ArtifactFileData, KnowledgeCitationData } from '@/weknora/renderers/registry';
import type { ConversationResultResources } from '@/weknora/conversations/ProductConversationMessages';
import { createProductConversationViewModel, createProductExecutionApi } from '@/weknora/conversations/view-model';
import { resolveProductSessionResources, type ProductSessionResourceSelection, type VerifiedProductSessionResources } from '@/weknora/conversations/resources';
import { createProductSessionAttachments } from '@/weknora/resources/product-session-attachments';
import { createMobileKnowledgeApi } from '@/weknora/knowledge/api';
import { createProductDictationPort } from '@/weknora/voice/native-dictation-port';
import { createProductVoiceTranscriber } from '@/weknora/voice/product-transcriber';
import { createProductVoiceSessionApi, createRealtimeVoiceSession, createUnavailableRealtimeVoicePort } from '@/weknora/voice/realtime';
import { createRunProgressPresenter } from '@/weknora/notifications/live-progress';
import { createExpoNotificationsProgressPort } from '@/weknora/notifications/native-progress-port';
import { useProductAuth } from '@/weknora/auth/session';
import { useMobileHost } from '@/weknora/platform/host';
import { createPersistentExecutionRequestStorage, getExecutionStorage } from '@/weknora/platform/execution-storage';
import { createProtocolGate } from '@/weknora/platform/protocol-gate';
import { projectExecutionSnapshot } from '@/weknora/conversations/execution-projection';

type Params = ProductSessionResourceSelection & { id?: string; resourceUserId?: string; resourceTenantId?: string; runId?: string };

function ProductSessionRoute() {
  const route = useRoute();
  const params = route.params as Partial<Params> | undefined;
  const sessionId = params?.id ?? '';
  const host = useMobileHost();
  const auth = useProductAuth();
  const identity = auth.scope.identity();
  const selection = params?.spaceId && params.agentId && params.targetId && params.workspaceRef
    ? { spaceId: params.spaceId, agentId: params.agentId, targetId: params.targetId, workspaceRef: params.workspaceRef }
    : null;
  // Navigation metadata is only a hint. The identity pair is emitted by the
  // server-owned ProductAuthSession and must match before any resource query
  // can make this a product route; no URL can select another tenant/user.
  const productRoute = selection !== null
    && params?.resourceUserId === identity.userId
    && params?.resourceTenantId === identity.tenantId
    && typeof params?.runId === 'string' && params.runId.trim() !== '';
  const [resources, setResources] = React.useState<VerifiedProductSessionResources | null>(null);
  const [resourceError, setResourceError] = React.useState<string | null>(null);
  React.useEffect(() => {
    if (!selection || !auth.authSession || !identity.userId || !identity.tenantId) return;
    const controller = new AbortController();
    setResources(null);
    setResourceError(null);
    void resolveProductSessionResources(auth.authSession, selection, identity, controller.signal)
      .then(setResources)
      .catch((error) => { if (!controller.signal.aborted) setResourceError(error instanceof Error ? error.message : 'PRODUCT_RESOURCES_UNAVAILABLE'); });
    return () => controller.abort();
  }, [auth.authSession, identity.tenantId, identity.userId, selection?.agentId, selection?.spaceId, selection?.targetId, selection?.workspaceRef]);
  const executionApi = React.useMemo(() => (
    host && auth.credential?.kind === 'bearer' && auth.authSession
      ? createProductExecutionApi({ origin: host.origin, credential: auth.credential, scope: auth.scope, authSession: auth.authSession })
      : null
  ), [auth.authSession, auth.credential, auth.scope, host]);
  const requestStorage = React.useMemo(() => {
    if (!identity.userId || !identity.tenantId || !host) return null;
    return createPersistentExecutionRequestStorage(AsyncStorage, `weknora:execution-request:${host.origin}:${identity.tenantId}:${identity.userId}:${sessionId}`);
  }, [host, identity.tenantId, identity.userId, sessionId]);
  const eventStorage = React.useMemo(() => {
    if (!identity.userId || !identity.tenantId || !host) return null;
    return getExecutionStorage({ origin: host.origin, tenantID: identity.tenantId, userID: identity.userId });
  }, [host, identity.tenantId, identity.userId]);
  // W37 protocol compatibility gate: the production handshake reads
  // /system/capabilities once per authenticated mount (over the same
  // auth-session transport as executions) and classifies this build against
  // the advertised window. Unknown schema, a failed handshake or an
  // out-of-window verdict stops cancel/steer at the view-model boundary
  // while login/reads and the upgrade notice stay available.
  const protocolGate = React.useMemo(() => (
    executionApi?.capabilities ? createProtocolGate((signal) => executionApi.capabilities!(signal)) : null
  ), [executionApi]);
  React.useEffect(() => {
    if (!protocolGate) return;
    const controller = new AbortController();
    void protocolGate.handshake(controller.signal).catch(() => undefined);
    return () => controller.abort();
  }, [protocolGate]);
  const viewModel = React.useMemo(() => {
    if (!resources || !executionApi || !sessionId || !requestStorage) return null;
    return createProductConversationViewModel({
      scope: auth.scope,
      spaceId: resources.spaceId,
      sessionId,
      agent: { id: resources.agentId, name: resources.agentName },
      targetId: resources.targetId,
      workspaceRef: resources.workspaceRef,
      budgetUpper: 0,
      executions: executionApi,
      requestStorage,
      eventStorage: eventStorage ?? undefined,
      projection: executionApi.snapshot ? {
        load: async (runID, signal) => projectExecutionSnapshot(await executionApi.snapshot!(runID, signal)),
      } : undefined,
      ...(protocolGate ? { protocolGate } : {}),
    });
  }, [auth.scope, executionApi, protocolGate, requestStorage, resources, sessionId]);
  // W25 upload chain (I-1 fix): the production assembly point. The pipeline
  // exists exactly when a bearer-authenticated product conversation does;
  // ConversationScreen keeps its own capability gate, so a route that cannot
  // assemble simply passes undefined and the entry stays hidden.
  const attachments = React.useMemo(() => {
    const credential = auth.credential;
    if (!host || !viewModel || !sessionId || credential?.kind !== 'bearer' || !auth.authSession) return null;
    return createProductSessionAttachments({
      origin: host.origin,
      credential,
      authSession: auth.authSession,
      scope: auth.scope,
      sessionID: sessionId,
    });
  }, [auth.authSession, auth.credential, auth.scope, host, sessionId, viewModel]);
  // W31 realtime voice chain: the session rides the W30 grant endpoints
  // (admission + idempotent settle) behind the product bearer, and a system
  // disconnect re-attaches through the W12 recovery controller instead of
  // reopening the paid session. The realtime media provider seam fails
  // closed (typed PROVIDER_UNAVAILABLE) until the native provider module
  // lands — the W29 pre-W30 typed-failure pattern — so controls, renewal
  // and approval policy ship behind a real grant lifecycle today.
  const voice = React.useMemo(() => {
    const credential = auth.credential;
    if (!host || !viewModel || credential?.kind !== 'bearer' || !auth.authSession) return null;
    const api = createProductVoiceSessionApi({ origin: host.origin, credential, authSession: auth.authSession });
    return {
      session: createRealtimeVoiceSession({
        port: createUnavailableRealtimeVoicePort(),
        admit: () => api.admit(sessionId, viewModel.execution?.runID ?? undefined),
        release: (id) => api.release(id),
        recover: () => viewModel.recovery?.recover() ?? Promise.resolve(),
      }),
    };
  }, [auth.authSession, auth.credential, host, sessionId, viewModel]);
  // W31 F-3: background run progress — the presenter consumes Run status
  // changes (status-only labels, no conversation content) through the
  // expo-notifications port (Live Activity stays declaration-only until its
  // native module lands; plain notifications degrade gracefully and a
  // missing permission posts nothing rather than faking a surface).
  const progress = React.useMemo(() => (
    host && viewModel ? createRunProgressPresenter(createExpoNotificationsProgressPort()) : null
  ), [host, viewModel]);
  // W29 voice chain: the production dictation port (expo-audio capture with
  // permission gating and temp-file cleanup). W30 wires the authenticated
  // transcription call into the port's consumption seam — the capture goes
  // to the product endpoint over the authSession's bearer, budget and the
  // provider key stay server-side; the scope seam cancels an in-flight
  // dictation on space switch.
  const dictation = React.useMemo(() => (
    host && viewModel && auth.credential?.kind === 'bearer' && auth.authSession
      ? { port: createProductDictationPort({
          transcribe: createProductVoiceTranscriber({ origin: host.origin, credential: auth.credential, authSession: auth.authSession }),
        }), scope: auth.scope }
      : null
  ), [auth.authSession, auth.credential, auth.scope, host, viewModel]);
  // W28 result resources: citations open and oversized analysis tables /
  // artifact files download by re-requesting authorization through the
  // product knowledge/attachment interfaces on every click (revocation
  // denies on the spot). Cross-space knowledge keeps the server's existing
  // share authorization and execution-space cost attribution — the client
  // never switches commercial accounts for a link.
  const resultResources = React.useMemo(() => {
    const credential = auth.credential;
    if (!host || credential?.kind !== 'bearer') return null;
    const knowledge = createMobileKnowledgeApi(host, credential);
    return {
      // 每次点击都重新走产品知识接口的授权链（服务端逐次校验，撤销即拒绝）。
      openCitation: async (citation: KnowledgeCitationData) => { await knowledge.detail(citation.documentID); },
      openFile: async (file: ArtifactFileData) => { await knowledge.download(file.ref); },
    } satisfies ConversationResultResources;
  }, [auth.credential, host]);
  // Sessions without explicit product resource metadata remain the retained Happy route.
  // Product sessions never receive defaults: they wait for all server ownership checks.
  if (!productRoute) return <SessionView id={sessionId} />;
  if (resourceError) return <View><Text accessibilityRole="alert">产品资源不可用：{resourceError}</Text></View>;
  if (!eventStorage) return <View><Text accessibilityRole="alert">产品会话需要原生加密事件存储，当前设备尚未完成初始化。</Text></View>;
  if (!viewModel) return <View><Text accessibilityRole="alert">正在验证产品会话资源…</Text></View>;
  // W12: the product conversation mounts its lifecycle recovery controller
  // (AppState active -> status/history/stream; background closes only the
  // subscription). The controller is per-view-model, i.e. per mount, matching
  // the screen's dispose-on-unmount contract.
  return <ConversationScreen sessionId={sessionId} viewModel={viewModel} attachments={attachments ?? undefined} recovery={viewModel.recovery} dictation={dictation ?? undefined} voice={voice ?? undefined} progress={progress ?? undefined} resultResources={resultResources ?? undefined} />;
}

export default React.memo(ProductSessionRoute);
