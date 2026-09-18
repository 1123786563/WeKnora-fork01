import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import { DataAnalysisResult, AnalysisFileFallback, ResultFileCard } from './DataAnalysisResult';
import { ANALYSIS_TABLE_LIMITS, parseArtifactFile, resolveAnalysisResult } from './registry';

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  return {
    Pressable: host('Pressable'), Text: host('Text'), TextInput: host('TextInput'), View: host('View'), ScrollView: host('ScrollView'),
  };
});

describe('DataAnalysisResult (W28)', () => {
  it('renders a bounded accessible table with typed cells', async () => {
    const resolution = resolveAnalysisResult({ columns: ['地区', '销售额'], rows: [['华东', 1200], ['华南', 980]] });
    expect(resolution.renderer).toBe('table');
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(DataAnalysisResult, { table: (resolution as any).table })); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'analysis-table' })).toBeDefined();
    expect(renderer!.root.findByProps({ accessibilityLabel: 'analysis-column-0' }).props.children).toBe('地区');
    expect(renderer!.root.findByProps({ accessibilityLabel: 'analysis-cell-1-1' }).props.children).toBe('980');
    await act(async () => renderer!.unmount());
  });

  it('clamps long chinese cells and keeps rows readable', async () => {
    const resolution = resolveAnalysisResult({ columns: ['摘要'], rows: [[('超长中文单元格内容'.repeat(40))]] });
    expect(resolution.renderer).toBe('table');
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(DataAnalysisResult, { table: (resolution as any).table })); });
    const cell = renderer!.root.findByProps({ accessibilityLabel: 'analysis-cell-0-0' });
    expect(cell.props.numberOfLines).toBe(1);
    await act(async () => renderer!.unmount());
  });

  it('falls back to an honest file card when the table exceeds render limits', async () => {
    const resolution = resolveAnalysisResult({
      columns: Array.from({ length: ANALYSIS_TABLE_LIMITS.maxColumns + 1 }, (_, i) => `c${i}`),
      rows: [[1]],
      artifact: { name: 'analysis.csv', mime: 'text/csv', bytes: 4096, ref: 'attachment-7' },
    });
    expect(resolution.renderer).toBe('file');
    const openFile = vi.fn(async () => undefined);
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(AnalysisFileFallback, { fallback: resolution as any, onDownload: openFile })); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'analysis-file-fallback' })).toBeDefined();
    expect(JSON.stringify(renderer!.toJSON())).toContain(String(ANALYSIS_TABLE_LIMITS.maxColumns + 1));
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '下载完整表格' }).props.onPress(); });
    expect(openFile).toHaveBeenCalledTimes(1);
    await act(async () => renderer!.unmount());
  });

  it('explains instead of faking a download when no artifact seam exists', async () => {
    const resolution = resolveAnalysisResult({ columns: ['a'], rows: Array.from({ length: ANALYSIS_TABLE_LIMITS.maxRows + 1 }, () => [1]) });
    expect(resolution.renderer).toBe('file');
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(AnalysisFileFallback, { fallback: resolution as any })); });
    expect(renderer!.root.findAllByProps({ accessibilityRole: 'button' })).toHaveLength(0);
    expect(JSON.stringify(renderer!.toJSON())).toContain('文件');
    await act(async () => renderer!.unmount());
  });

  it('renders artifact file rows through the authorized open seam', async () => {
    const file = parseArtifactFile({ name: 'result.csv', mime: 'text/csv', bytes: 12, ref: 'attachment-9' });
    const openFile = vi.fn(async () => undefined);
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(ResultFileCard, { file, onOpen: openFile })); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'artifact-file-attachment-9' })).toBeDefined();
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '打开文件 result.csv' }).props.onPress(); });
    expect(openFile).toHaveBeenCalledTimes(1);
    await act(async () => renderer!.unmount());
  });
});
