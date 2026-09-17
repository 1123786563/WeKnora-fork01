import * as React from 'react';
import { Pressable, Text, View } from 'react-native';
import type { KnowledgeCitationData } from './registry';

/**
 * W28 — 知识引用卡片。
 *
 * 只消费 `parseKnowledgeCitation` 校验过的契约数据：数据里没有任何可执行
 * 的 source URL。卡片也不是授权层——每次点击都通过 `onOpen` seam 重新向
 * 产品知识接口请求授权（服务端逐次校验，撤销即拒绝），seam 未接线时显示
 * 产品入口说明而不是伪造一次“已打开”。
 */
export interface KnowledgeCitationProps {
  citation: KnowledgeCitationData;
  /** 每次点击重新请求授权并打开获准来源的产品资源 seam。 */
  onOpen?: (citation: KnowledgeCitationData) => Promise<void> | void;
}

export function KnowledgeCitation({ citation, onOpen }: KnowledgeCitationProps) {
  const [state, setState] = React.useState<'idle' | 'opening' | 'opened' | 'denied'>('idle');
  const [error, setError] = React.useState<string | null>(null);
  const open = async () => {
    if (!onOpen) return;
    setState('opening');
    setError(null);
    try {
      await onOpen(citation);
      setState('opened');
    } catch (cause) {
      // 撤销授权、过期票据等一切失败都在这里显式呈现，绝不静默吞掉。
      setState('denied');
      setError(cause instanceof Error ? cause.message : 'CITATION_OPEN_FAILED');
    }
  };
  return (
    <View accessibilityLabel={`knowledge-citation-${citation.documentID}`} style={{ paddingVertical: 6, gap: 4 }}>
      <Text accessibilityLabel={`citation-title-${citation.documentID}`} numberOfLines={2} style={{ fontWeight: '600' }}>
        {citation.title}
      </Text>
      {citation.snippet ? <Text numberOfLines={3}>{citation.snippet}</Text> : null}
      <Text accessibilityLabel={`citation-source-${citation.documentID}`} numberOfLines={1}>
        {`知识来源 ${citation.knowledgeID ?? citation.documentID}${citation.chunkIDs.length > 0 ? ` · ${citation.chunkIDs.length} 个片段` : ''}`}
      </Text>
      {onOpen ? (
        <Pressable accessibilityRole="button" accessibilityLabel={`打开引用 ${citation.title}`} onPress={() => void open()} disabled={state === 'opening'}>
          <Text>{state === 'opening' ? '正在验证授权…' : '打开引用'}</Text>
        </Pressable>
      ) : (
        <Text>{'引用来源需通过产品知识入口打开'}</Text>
      )}
      {state === 'opened' ? (
        <Text accessibilityLabel={`citation-opened-${citation.documentID}`}>已打开获准来源</Text>
      ) : null}
      {state === 'denied' && error ? (
        <Text accessibilityRole="alert">{`引用来源打开被拒绝：${error}`}</Text>
      ) : null}
    </View>
  );
}
