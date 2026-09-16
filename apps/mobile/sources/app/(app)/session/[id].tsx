import * as React from 'react';
import { Text, View } from 'react-native';
import { useRoute } from '@react-navigation/native';
import { ConversationScreen } from '@/weknora/conversations/ConversationScreen';
import { createProductConversationViewModel, createProductExecutionApi } from '@/weknora/conversations/view-model';
import { useProductAuth } from '@/weknora/auth/session';
import { useMobileHost } from '@/weknora/platform/host';

function ProductSessionRoute() {
  const route = useRoute();
  const params = route.params as { id?: string; spaceId?: string; agentId?: string; targetId?: string; workspaceRef?: string } | undefined;
  const sessionId = params?.id ?? '';
  const host = useMobileHost();
  const auth = useProductAuth();
  const executionApi = React.useMemo(() => (
    host && auth.credential?.kind === 'bearer'
      ? createProductExecutionApi({ origin: host.origin, credential: auth.credential, scope: auth.scope })
      : null
  ), [auth.credential, auth.scope, host]);
  const viewModel = React.useMemo(() => {
    if (!executionApi || !sessionId) return null;
    return createProductConversationViewModel({
      scope: auth.scope,
      spaceId: params?.spaceId ?? auth.scope.identity().tenantId,
      sessionId,
      agent: { id: params?.agentId ?? 'default', name: 'Default Agent' },
      targetId: params?.targetId ?? sessionId,
      workspaceRef: params?.workspaceRef ?? `mobile:${sessionId}`,
      budgetUpper: 0,
      executions: executionApi,
    });
  }, [auth.scope, executionApi, params?.agentId, params?.spaceId, params?.targetId, params?.workspaceRef, sessionId]);
  if (!viewModel) return <View><Text accessibilityRole="alert">产品会话尚未准备好</Text></View>;
  return <ConversationScreen sessionId={sessionId} viewModel={viewModel} />;
}

export default React.memo(ProductSessionRoute);
