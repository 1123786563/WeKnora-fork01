import * as React from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';
import type { AnalysisTableCell, AnalysisTableData, ArtifactFileData } from './registry';

/**
 * W28 — 数据分析表格 / 文件回退 / 产物文件卡片。
 *
 * 只消费 `resolveAnalysisResult` / `parseArtifactFile` 校验过的数据。表格
 * 渲染有界：单元格单行截断、无固定高度（大字体安全）、横向 ScrollView 承
 * 接宽表（RTL 由系统布局镜像）。超限表格降级为文件卡片，下载动作经注入
 * seam 走产品附件/知识接口重新请求授权；没有可下载对象或没有 seam 时显示
 * 真实摘要说明，不以空壳卡片冒充完成。
 */

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export interface DataAnalysisResultProps {
  table: AnalysisTableData;
}

export function DataAnalysisResult({ table }: DataAnalysisResultProps) {
  return (
    <View accessibilityLabel="analysis-table" style={{ paddingVertical: 6, gap: 4 }}>
      <ScrollView horizontal>
        <View style={{ gap: 2 }}>
          <View accessibilityLabel="analysis-header" style={{ flexDirection: 'row', gap: 8 }}>
            {table.columns.map((column, index) => (
              <Text key={`${column}-${index}`} accessibilityLabel={`analysis-column-${index}`} numberOfLines={1} style={{ fontWeight: '600', minWidth: 64 }}>
                {column}
              </Text>
            ))}
          </View>
          {table.rows.map((row, rowIndex) => (
            <View key={`row-${rowIndex}`} accessibilityLabel={`analysis-row-${rowIndex}`} style={{ flexDirection: 'row', gap: 8 }}>
              {row.map((cell: AnalysisTableCell, columnIndex) => (
                <Text key={`cell-${rowIndex}-${columnIndex}`} accessibilityLabel={`analysis-cell-${rowIndex}-${columnIndex}`} numberOfLines={1} style={{ minWidth: 64 }}>
                  {cell === null ? '—' : String(cell)}
                </Text>
              ))}
            </View>
          ))}
        </View>
      </ScrollView>
    </View>
  );
}

export interface AnalysisFileFallbackData {
  reason: 'TOO_MANY_COLUMNS' | 'TOO_MANY_ROWS' | 'CELL_TOO_LARGE';
  columns: number;
  rows: number;
  file?: ArtifactFileData;
}

const fallbackReasonText: Record<AnalysisFileFallbackData['reason'], string> = {
  TOO_MANY_COLUMNS: '列数超过渲染上限',
  TOO_MANY_ROWS: '行数超过渲染上限',
  CELL_TOO_LARGE: '单元格内容超过渲染上限',
};

export interface AnalysisFileFallbackProps {
  fallback: AnalysisFileFallbackData;
  /** 下载动作的产品资源 seam（对 fallback.file 重新请求授权）。 */
  onDownload?: () => Promise<void> | void;
}

export function AnalysisFileFallback({ fallback, onDownload }: AnalysisFileFallbackProps) {
  const [state, setState] = React.useState<'idle' | 'downloading' | 'failed'>('idle');
  const [error, setError] = React.useState<string | null>(null);
  const download = async () => {
    if (!onDownload) return;
    setState('downloading');
    setError(null);
    try {
      await onDownload();
      setState('idle');
    } catch (cause) {
      setState('failed');
      setError(cause instanceof Error ? cause.message : 'ANALYSIS_DOWNLOAD_FAILED');
    }
  };
  return (
    <View accessibilityLabel="analysis-file-fallback" style={{ paddingVertical: 6, gap: 4 }}>
      <Text style={{ fontWeight: '600' }}>{'分析结果已切换为文件'}</Text>
      <Text>{`${fallbackReasonText[fallback.reason]}（${fallback.columns} 列 × ${fallback.rows} 行），完整结果以文件形式提供。`}</Text>
      {fallback.file ? (
        <Text numberOfLines={1}>{`${fallback.file.name} · ${formatBytes(fallback.file.bytes)} · ${fallback.file.mime}`}</Text>
      ) : null}
      {fallback.file && onDownload ? (
        <Pressable accessibilityRole="button" accessibilityLabel="下载完整表格" onPress={() => void download()} disabled={state === 'downloading'}>
          <Text>{state === 'downloading' ? '正在下载…' : '下载完整表格'}</Text>
        </Pressable>
      ) : (
        <Text>{'该结果未携带可下载的产品附件引用'}</Text>
      )}
      {state === 'failed' && error ? (
        <Text accessibilityRole="alert">{`文件下载失败：${error}`}</Text>
      ) : null}
    </View>
  );
}

export interface ResultFileCardProps {
  file: ArtifactFileData;
  /** 打开/下载动作的产品资源 seam（对 file.ref 重新请求授权）。 */
  onOpen?: (file: ArtifactFileData) => Promise<void> | void;
}

export function ResultFileCard({ file, onOpen }: ResultFileCardProps) {
  const [state, setState] = React.useState<'idle' | 'opening' | 'failed'>('idle');
  const [error, setError] = React.useState<string | null>(null);
  const open = async () => {
    if (!onOpen) return;
    setState('opening');
    setError(null);
    try {
      await onOpen(file);
      setState('idle');
    } catch (cause) {
      setState('failed');
      setError(cause instanceof Error ? cause.message : 'ARTIFACT_OPEN_FAILED');
    }
  };
  return (
    <View accessibilityLabel={`artifact-file-${file.ref}`} style={{ paddingVertical: 6, gap: 4 }}>
      <Text numberOfLines={1} style={{ fontWeight: '600' }}>{file.name}</Text>
      <Text numberOfLines={1}>{`${formatBytes(file.bytes)} · ${file.mime}`}</Text>
      {onOpen ? (
        <Pressable accessibilityRole="button" accessibilityLabel={`打开文件 ${file.name}`} onPress={() => void open()} disabled={state === 'opening'}>
          <Text>{state === 'opening' ? '正在验证授权…' : '打开文件'}</Text>
        </Pressable>
      ) : (
        <Text>{'文件需通过产品附件入口打开'}</Text>
      )}
      {state === 'failed' && error ? (
        <Text accessibilityRole="alert">{`文件打开被拒绝：${error}`}</Text>
      ) : null}
    </View>
  );
}
