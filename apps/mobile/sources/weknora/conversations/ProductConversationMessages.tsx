import * as React from 'react';
import { ScrollView, Text, View } from 'react-native';
import type { ConversationViewModel, ConversationContentBlock } from './view-model';
import { KnowledgeCitation } from '../renderers/KnowledgeCitation';
import { AnalysisFileFallback, DataAnalysisResult, ResultFileCard } from '../renderers/DataAnalysisResult';
import { parseStructuredResultText, routeStructuredResult, type ArtifactFileData, type KnowledgeCitationData } from '../renderers/registry';

/**
 * W28 — 产品消息面上的授权资源 seam。打开引用、下载超限表格/产物文件都
 * 在每次点击时经这些入口重新请求授权（复用产品知识/附件接口，W25 管线）；
 * 渲染器与消息面都不是授权层。跨空间来源沿用服务端既有共享授权与费用
 * 归属，客户端不自行切换商业账户。
 */
export interface ConversationResultResources {
  /** 经产品知识接口重新请求授权并打开获准来源；撤销后此处拒绝。 */
  openCitation(citation: KnowledgeCitationData): Promise<void>;
  /** 经产品附件/知识接口重新请求授权并打开/下载文件对象。 */
  openFile(file: ArtifactFileData): Promise<void>;
}

/** ConversationScreen 在挂载点提供 seam；SessionView 内的消息面从这里读取。 */
export const ConversationResultResourcesContext = React.createContext<ConversationResultResources | null>(null);

/** One message block routed through the W28 registry. */
function ResultBlock({ block, resources }: { block: ConversationContentBlock; resources: ConversationResultResources | null }) {
  const envelope = block.kind === 'tool' ? parseStructuredResultText(block.text) : null;
  if (!envelope) {
    return <Text accessibilityLabel={`message-block-${block.id ?? 'tool'}`}>{block.text}</Text>;
  }
  const routed = routeStructuredResult(envelope);
  switch (routed.renderer) {
    case 'citation':
      return <KnowledgeCitation citation={routed.citation} onOpen={resources?.openCitation} />;
    case 'table':
      return <DataAnalysisResult table={routed.analysis.table} />;
    case 'file':
      return 'analysis' in routed
        ? <AnalysisFileFallback
            fallback={routed.analysis}
            onDownload={routed.analysis.file && resources ? () => resources.openFile(routed.analysis.file!) : undefined}
          />
        : <ResultFileCard file={routed.file} onOpen={resources?.openFile} />;
    default:
      // 未知类型与损坏 payload 都退回安全文本：绝不出现空壳卡片。
      return <Text accessibilityLabel={`message-block-${block.id ?? 'tool'}`}>{block.text}</Text>;
  }
}

/** Product message projection. It deliberately consumes only VM messages. */
export function ProductConversationMessages({ viewModel, resources: resourcesProp }: { viewModel: ConversationViewModel; resources?: ConversationResultResources | null }) {
  const contextResources = React.useContext(ConversationResultResourcesContext);
  const resources = resourcesProp ?? contextResources;
  return (
    <ScrollView accessibilityLabel="product-conversation-messages" contentContainerStyle={{ padding: 16, gap: 10 }}>
      {viewModel.messages.map((message) => (
        <View key={message.id} accessibilityLabel={`message-${message.id}`}>
          <Text accessibilityLabel={`message-role-${message.id}`}>{message.role}</Text>
          {message.agentID ? (
            <Text accessibilityLabel={`message-agent-${message.id}`}>{`Agent ${message.agentID}`}</Text>
          ) : null}
          {(message.blocks?.length ? message.blocks : [{ kind: 'text' as const, text: message.text }]).map((block, index) => (
            <View key={block.id ?? `${message.id}-${block.kind}-${index}`} accessibilityLabel={`message-${message.id}-${block.kind}`}>
              <ResultBlock block={block} resources={resources} />
            </View>
          ))}
        </View>
      ))}
      {viewModel.execution ? (
        <View accessibilityLabel="execution-state">
          <Text>{viewModel.execution.status}</Text>
          {viewModel.execution.reason ? <Text>{viewModel.execution.reason}</Text> : null}
        </View>
      ) : null}
    </ScrollView>
  );
}
