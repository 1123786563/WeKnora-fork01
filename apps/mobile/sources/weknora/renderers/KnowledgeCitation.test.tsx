import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import { KnowledgeCitation } from './KnowledgeCitation';
import { parseKnowledgeCitation } from './registry';

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  return {
    Pressable: host('Pressable'), Text: host('Text'), TextInput: host('TextInput'), View: host('View'), ScrollView: host('ScrollView'),
  };
});

const citation = parseKnowledgeCitation({
  document_id: 'doc-1', knowledge_id: 'kb-1', chunk_ids: ['c1', 'c2'],
  title: '产品知识手册', snippet: '这是一段很长的中文摘录，用于验证引用卡片在长文本下的展示与截断行为。',
});

describe('KnowledgeCitation (W28)', () => {
  it('renders an accessible citation card and never the raw source url', async () => {
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(KnowledgeCitation, { citation, onOpen: async () => undefined })); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'knowledge-citation-doc-1' })).toBeDefined();
    expect(renderer!.root.findByProps({ accessibilityRole: 'button', accessibilityLabel: '打开引用 产品知识手册' })).toBeDefined();
    // The parser dropped source_url; the tree never contains an executable link.
    expect(JSON.stringify(renderer!.toJSON())).not.toContain('https://');
    await act(async () => renderer!.unmount());
  });

  it('re-requests authorization on every press and opens the approved source', async () => {
    const onOpen = vi.fn(async () => undefined);
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(KnowledgeCitation, { citation, onOpen })); });
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '打开引用 产品知识手册' }).props.onPress(); });
    // The renderer is not the authorization layer: pressing again must call
    // the seam again (per-click re-request), not trust the first grant.
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '打开引用 产品知识手册' }).props.onPress(); });
    expect(onOpen).toHaveBeenCalledTimes(2);
    expect(renderer!.root.findByProps({ accessibilityLabel: 'citation-opened-doc-1' })).toBeDefined();
    await act(async () => renderer!.unmount());
  });

  it('shows the denial after the authorization is revoked', async () => {
    let authorized = true;
    const onOpen = vi.fn(async () => { if (!authorized) throw new Error('HTTP_403'); });
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(KnowledgeCitation, { citation, onOpen })); });
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '打开引用 产品知识手册' }).props.onPress(); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'citation-opened-doc-1' })).toBeDefined();
    authorized = false;
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '打开引用 产品知识手册' }).props.onPress(); });
    expect(onOpen).toHaveBeenCalledTimes(2);
    expect(renderer!.root.findByProps({ accessibilityRole: 'alert' }).props.children).toContain('HTTP_403');
    await act(async () => renderer!.unmount());
  });

  it('bounds long chinese titles and snippets for large fonts and stays pressable without a seam', async () => {
    const longTitle = '长标题'.repeat(60);
    const longSnippet = '长中文摘录内容'.repeat(120);
    const parsed = parseKnowledgeCitation({ document_id: 'doc-2', chunk_ids: ['c'], title: longTitle, snippet: longSnippet });
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(KnowledgeCitation, { citation: parsed })); });
    // Title and snippet are clamped by numberOfLines; no fixed heights are set.
    const boundedTexts = renderer!.root.findAll((node: { props?: { numberOfLines?: unknown } }) => node.props?.numberOfLines !== undefined);
    expect(boundedTexts.length).toBeGreaterThanOrEqual(2);
    expect(JSON.stringify(renderer!.toJSON())).not.toContain(longSnippet);
    // Without a product resource seam the card explains the entry instead of
    // faking an authorized open.
    expect(renderer!.root.findAllByProps({ accessibilityRole: 'button' })).toHaveLength(0);
    expect(JSON.stringify(renderer!.toJSON())).toContain('产品知识入口');
    await act(async () => renderer!.unmount());
  });
});
