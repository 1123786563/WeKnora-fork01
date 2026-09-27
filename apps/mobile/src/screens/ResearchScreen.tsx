import { createElement, useState } from 'react';
import { Text, TextInput, View } from 'react-native';
import type { MaterialEntry, ResearchAnnotationRow, ResearchDelegationRow } from '@weknora/mobile-core';
import type { ResearchViewState } from '../research-view.ts';

export interface ResearchScreenProps {
  state: ResearchViewState;
  onDelegate(objective: string, sources: string): void;
  onAnnotate(input: { materialId: string; baseVersion: string; body: string }): void;
  onFlushDrafts(): void;
  onRequestRevision(input: { materialId: string; baseVersion: string; note: string; action: 'steer' | 'queue_next'; expectedRevision: number }): void;
  onRefresh(): void;
  onBack(): void;
}

/** 研究屏（演示态）：委派表单 + 委派/批注投影 + 版本钉定批注表单 + 修订请求表单。
 *  只消费 TaskResearch Interface；wire/契约/scope 纪律全部在模块后。 */
export function ResearchScreen({ state, onDelegate, onAnnotate, onFlushDrafts, onRequestRevision, onRefresh, onBack }: ResearchScreenProps) {
  const [objective, setObjective] = useState('');
  const [sources, setSources] = useState('');
  const [materialId, setMaterialId] = useState('');
  const [baseVersion, setBaseVersion] = useState('');
  const [body, setBody] = useState('');
  const [note, setNote] = useState('');
  const [expectedRevision, setExpectedRevision] = useState('0');
  const [action, setAction] = useState<'steer' | 'queue_next'>('queue_next');
  const materials: MaterialEntry[] = state.materials ?? [];
  return createElement(
    View,
    { style: { padding: 16, gap: 12 } },
    createElement(Text, { style: { fontSize: 20, fontWeight: '600' } }, '研究与批注'),
    createElement(Text, { onPress: onBack }, '← 返回'),
    createElement(Text, { onPress: onRefresh }, '刷新'),
    state.loading === true && createElement(Text, null, '加载中…'),
    state.error !== undefined && createElement(Text, { testID: 'research-error' }, state.error),
    state.notice !== undefined && createElement(Text, { testID: 'research-notice' }, state.notice),
    (state.pendingDrafts ?? 0) > 0 && createElement(Text, { onPress: onFlushDrafts, testID: 'research-flush' }, `离线批注草稿 ${state.pendingDrafts} 条 · 点此同步`),
    createElement(Text, { style: { fontWeight: '600' } }, '委派只读研究'),
    createElement(TextInput, { placeholder: '研究目标', value: objective, onChangeText: setObjective, testID: 'research-objective' }),
    createElement(TextInput, { placeholder: '来源知识库（逗号分隔）', value: sources, onChangeText: setSources, testID: 'research-sources' }),
    createElement(Text, { onPress: () => onDelegate(objective, sources), testID: 'research-delegate' }, '委派（只读，不占用写运行）'),
    (state.delegations ?? []).map((delegation) =>
      createElement(Text, { key: delegation.delegationId }, `${delegation.status === 'completed' ? '已完成' : '进行中'} · ${delegation.objective} · 来源 ${delegation.sources.join('、')}${delegation.summary === undefined ? '' : ' · ' + delegation.summary}`),
    ),
    createElement(Text, { style: { fontWeight: '600' } }, '批注材料版本（生成新批注记录，原版本不变）'),
    materials.map((entry) =>
      createElement(Text, { key: entry.materialId, onPress: () => { setMaterialId(entry.materialId); setBaseVersion(entry.version); } }, `${entry.name} · 版本 ${entry.version}`),
    ),
    createElement(TextInput, { placeholder: '材料（如 m1:0）', value: materialId, onChangeText: setMaterialId, testID: 'research-material' }),
    createElement(TextInput, { placeholder: '当前版本（从材料列表点选）', value: baseVersion, onChangeText: setBaseVersion, testID: 'research-version' }),
    createElement(TextInput, { placeholder: '批注内容', value: body, onChangeText: setBody, testID: 'research-body', multiline: true }),
    createElement(Text, { onPress: () => onAnnotate({ materialId, baseVersion, body }), testID: 'research-annotate' }, '提交批注'),
    (state.annotations ?? []).map((annotation) =>
      createElement(Text, { key: annotation.annotationId }, `${annotation.materialId} @ ${annotation.baseVersion} · ${annotation.body}`),
    ),
    createElement(Text, { style: { fontWeight: '600' } }, '请求修订（基于指定版本生成新版本）'),
    createElement(TextInput, { placeholder: '修订说明', value: note, onChangeText: setNote, testID: 'research-revision-note' }),
    createElement(TextInput, { placeholder: '任务当前 revision（详情页可见）', value: expectedRevision, onChangeText: setExpectedRevision, testID: 'research-revision-num', inputMode: 'numeric' }),
    createElement(Text, { onPress: () => setAction(action === 'steer' ? 'queue_next' : 'steer') }, `通道：${action === 'steer' ? '注入当前运行' : '排队下一次运行'}`),
    createElement(
      Text,
      {
        onPress: () => onRequestRevision({ materialId, baseVersion, note, action, expectedRevision: Number(expectedRevision) || 0 }),
        testID: 'research-revision',
      },
      '请求修订',
    ),
  );
}
