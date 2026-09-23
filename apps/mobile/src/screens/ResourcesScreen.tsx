import { Button, Text, View } from 'react-native';
import { scanStatusPresentation } from '@weknora/domain/mobile';
import type { ResourcePage } from '@weknora/mobile-core';

export interface ResourcesScreenProps {
  page?: ResourcePage;
  loading: boolean;
  error?: string;
  onRefresh(): void;
}

const CLASS_TITLES = { agent: 'Agents', knowledge: 'Knowledge', connection: 'Connections' } as const;
const CAPABILITY_LABELS = { supported: 'Available', unavailable: 'Unavailable', forbidden: 'Access revoked' } as const;

function capabilitySuffix(state: 'supported' | 'unavailable' | 'forbidden', reason: string): string {
  return state === 'supported' ? '' : ` — ${CAPABILITY_LABELS[state]} (${reason})`;
}

/** Resources 页：只消费 Resource Shelf Interface 的投影（AC2）；三态必须解释原因，撤权类不渲染行。 */
export function ResourcesScreen({ page, loading, error, onRefresh }: ResourcesScreenProps) {
  if (!page) {
    return (
      <View>
        <Text>{error ?? (loading ? 'Loading resources…' : 'No resources')}</Text>
        {error ? <Button title="Retry" onPress={onRefresh} /> : null}
      </View>
    );
  }
  return (
    <View>
      <Text>{`Resources for tenant ${page.tenantId}`}</Text>
      <Button title="Refresh" onPress={onRefresh} />
      {(['agent', 'knowledge', 'connection'] as const).flatMap((resourceClass) => {
        const verdict = page.classVerdicts[resourceClass];
        if (verdict.state === 'supported') return [];
        return [<Text key={`${resourceClass}-verdict`}>{`${CLASS_TITLES[resourceClass]}: ${CAPABILITY_LABELS[verdict.state]} (${verdict.reason})`}</Text>];
      })}
      {page.agents.map((agent) => (
        <Text key={agent.id}>{`${agent.name}${capabilitySuffix(agent.capability.state, agent.capability.reason)}`}</Text>
      ))}
      {page.knowledge.map((resource) => (
        <Text key={resource.id}>{`${resource.title} — ${scanStatusPresentation(resource.scanStatus).label}`}</Text>
      ))}
      {page.connections.map((connection) => (
        <Text key={connection.id}>{`${connection.kind} connection — ${connection.connected ? 'connected' : connection.capability.reason}`}</Text>
      ))}
    </View>
  );
}
