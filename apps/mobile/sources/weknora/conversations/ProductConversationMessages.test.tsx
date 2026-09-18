import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import { ProductConversationMessages } from './ProductConversationMessages';
import type { ConversationViewModel } from './view-model';

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  return {
    Pressable: host('Pressable'), Text: host('Text'), TextInput: host('TextInput'), View: host('View'), ScrollView: host('ScrollView'),
  };
});

function model(messages: ConversationViewModel['messages']): ConversationViewModel {
  return {
    scope: { origin: 'https://api.example', userId: 'u1', tenantId: 't1', spaceId: 's1' },
    messages,
    pendingInteractions: [],
    capabilities: { canCancel: false, canSteer: false, canAttach: false, canVoice: false },
    execution: null,
    commands: { cancel: async () => undefined, steer: async () => undefined },
  };
}

describe('ProductConversationMessages structured results (W28)', () => {
  it('routes a knowledge citation tool block through the citation renderer', async () => {
    const openCitation = vi.fn(async () => undefined);
    const viewModel = model([
      { id: 'm1', role: 'assistant', text: '', agentID: 'agent-7', blocks: [
        { id: 'b1', kind: 'tool', text: JSON.stringify({ type: 'knowledge.citation', data: { document_id: 'doc-1', chunk_ids: ['c1'] } }) },
      ] },
    ]);
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(ProductConversationMessages, {
      viewModel,
      resources: { openCitation, openFile: async () => undefined },
    })); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'knowledge-citation-doc-1' })).toBeDefined();
    // 每轮 Agent ID 随消息保存并在消息面上可见。
    expect(renderer!.root.findByProps({ accessibilityLabel: 'message-agent-m1' }).props.children).toBe('Agent agent-7');
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '打开引用 知识引用' }).props.onPress(); });
    expect(openCitation).toHaveBeenCalledTimes(1);
    await act(async () => renderer!.unmount());
  });

  it('routes an analysis table and keeps plain text and tool text rendering intact', async () => {
    const viewModel = model([
      { id: 'm1', role: 'assistant', text: '普通文本块', blocks: [
        { id: 'b1', kind: 'text', text: '普通文本块' },
        { id: 'b2', kind: 'tool', text: '原始工具输出，不是 JSON' },
      ] },
      { id: 'm2', role: 'assistant', text: '', blocks: [
        { id: 'b3', kind: 'tool', text: JSON.stringify({ type: 'analysis.table', data: { columns: ['地区', '销售额'], rows: [['华东', 1200]] } }) },
      ] },
    ]);
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(ProductConversationMessages, { viewModel })); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'message-m1-text' })).toBeDefined();
    // 非 JSON 的 tool 块仍是安全文本，不进入注册器。
    expect(renderer!.root.findByProps({ accessibilityLabel: 'message-m1-tool' })).toBeDefined();
    expect(JSON.stringify(renderer!.toJSON())).toContain('原始工具输出');
    expect(renderer!.root.findByProps({ accessibilityLabel: 'analysis-table' })).toBeDefined();
    expect(renderer!.root.findByProps({ accessibilityLabel: 'analysis-cell-0-1' }).props.children).toBe('1200');
    await act(async () => renderer!.unmount());
  });

  it('degrades unknown types and corrupt payloads to safe text without shell cards', async () => {
    const viewModel = model([
      { id: 'm1', role: 'assistant', text: '', blocks: [
        { id: 'b1', kind: 'tool', text: JSON.stringify({ type: 'remote-component', data: { component: 'chart' } }) },
        { id: 'b2', kind: 'tool', text: '{"type":"knowledge.citation","data":{"source_url":"https://evil.example"}}' },
        { id: 'b3', kind: 'tool', text: '{"type":"artifact.file","data":{"name":"x.csv"' },
      ] },
    ]);
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(ProductConversationMessages, { viewModel })); });
    const rendered = JSON.stringify(renderer!.toJSON());
    // 未接通的专业类型不以空壳卡片出现：只保留安全文本。
    expect(renderer!.root.findAllByProps({ accessibilityLabel: 'knowledge-citation-https://evil.example' })).toHaveLength(0);
    expect(rendered).toContain('remote-component');
    expect(rendered).toContain('source_url');
    expect(renderer!.root.findAllByProps({ accessibilityRole: 'button' })).toHaveLength(0);
    await act(async () => renderer!.unmount());
  });

  it('renders an oversized analysis attachment as the authorized file fallback', async () => {
    const openFile = vi.fn(async () => undefined);
    const wide = Array.from({ length: 13 }, (_, i) => `c${i}`);
    const viewModel = model([
      { id: 'm1', role: 'assistant', text: '', blocks: [
        { id: 'b1', kind: 'tool', text: JSON.stringify({ type: 'analysis.table', data: { columns: wide, rows: [[1]], artifact: { name: 'analysis.csv', mime: 'text/csv', bytes: 4096, ref: 'attachment-7' } } }) },
      ] },
    ]);
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(ProductConversationMessages, {
      viewModel,
      resources: { openCitation: async () => undefined, openFile },
    })); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'analysis-file-fallback' })).toBeDefined();
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '下载完整表格' }).props.onPress(); });
    expect(openFile).toHaveBeenCalledTimes(1);
    await act(async () => renderer!.unmount());
  });
});
