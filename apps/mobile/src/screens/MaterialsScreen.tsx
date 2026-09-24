import { Button, Image, ScrollView, Text, View } from 'react-native';
import { bytesToBase64 } from '@weknora/mobile-core';
import type { MaterialIndex, MaterialView } from '@weknora/mobile-core';

export interface MaterialsScreenProps {
  index?: MaterialIndex;
  view?: MaterialView;
  loading: boolean;
  error?: string;
  grant?: { name: string; url: string; expiresAt: string };
  onOpenMaterial(materialId: string): void;
  onOpenTerminal(): void;
  onOpenEvidence(): void;
  onDownload(materialId: string): void;
  onShare(materialId: string): void;
  onRefresh(): void;
  onBack(): void;
}

const KIND_LABELS = { artifact: '产物', diff: '变更', 'test-report': '测试报告' } as const;

/** 材料屏：索引列表 + 视图面板。终端恒只读（无输入控件）；不支持/大文件给下载与分享路径。 */
export function MaterialsScreen({ index, view, loading, error, grant, onOpenMaterial, onOpenTerminal, onOpenEvidence, onDownload, onShare, onRefresh, onBack }: MaterialsScreenProps) {
  return (
    <ScrollView>
      <Button title="返回任务" onPress={onBack} />
      <Text>任务材料</Text>
      <Button title="刷新材料" onPress={onRefresh} disabled={loading} />
      {index === undefined
        ? <Text>{loading ? '正在读取材料索引…' : '尚无材料索引'}</Text>
        : (
          <View>
            {index.materials.map((entry) => (
              <View key={entry.materialId}>
                <Text>{entry.name}</Text>
                <Text>{KIND_LABELS[entry.kind]} · {entry.mime} · {entry.size} B · 版本 {entry.version}</Text>
                <Button title="打开" onPress={() => onOpenMaterial(entry.materialId)} />
                <Button title="下载" onPress={() => onDownload(entry.materialId)} />
                <Button title="分享" onPress={() => onShare(entry.materialId)} />
              </View>
            ))}
            {index.terminal.available && <Button title="只读终端" onPress={onOpenTerminal} />}
            <Button title="证据引用" onPress={onOpenEvidence} />
          </View>
        )}
      {grant !== undefined && (
        <View>
          <Text>下载授权（短时效，请尽快使用）</Text>
          <Text>{grant.name} · 有效至 {grant.expiresAt.replace('T', ' ').replace('Z', ' UTC')}</Text>
          <Text>{grant.url}</Text>
        </View>
      )}
      {view !== undefined && <MaterialViewPane view={view} />}
      {error !== undefined && <Text>{error}</Text>}
    </ScrollView>
  );
}

function MaterialViewPane({ view }: { view: MaterialView }) {
  if (view.kind === 'terminal') {
    return (
      <View>
        <Text>终端（只读）</Text>
        {view.lines.map((line) => (
          <Text key={line.seq}>{line.stream === 'stderr' ? '[stderr] ' : ''}{line.text}</Text>
        ))}
        {view.nextCursor !== undefined && <Text>可继续加载（游标 {view.nextCursor}）</Text>}
      </View>
    );
  }
  if (view.kind === 'evidence') {
    return (
      <View>
        <Text>证据引用</Text>
        {view.citations.map((citation) => (
          <View key={citation.seq}>
            <Text>#{citation.seq} · {citation.type}{citation.source === undefined ? '' : ` · 来源 ${citation.source}`}</Text>
            <Text>{citation.occurredAt.replace('T', ' ').replace('Z', ' UTC')}</Text>
          </View>
        ))}
        {view.citations.length === 0 && <Text>该任务暂无可追溯的工具与产物来源。</Text>}
      </View>
    );
  }
  if (view.kind === 'diff') {
    if (view.preview.state === 'unsupported') return <Text>该变更文件{view.preview.reason === 'size' ? '过大' : '类型不支持'}内联展示，请下载后查看。</Text>;
    if (view.malformed) {
      return (
        <View>
          <Text>无法解析为标准 diff，以下为原始内容：</Text>
          {view.raw !== undefined && <Text>{view.raw}</Text>}
        </View>
      );
    }
    return (
      <View>
        {view.hunks.map((hunk) => (
          <View key={hunk.header}>
            <Text>{hunk.header}</Text>
            {hunk.lines.map((line, position) => (
              <Text key={position}>{line.origin === 'add' ? '+' : line.origin === 'remove' ? '-' : ' '}{line.text}</Text>
            ))}
          </View>
        ))}
      </View>
    );
  }
  if (view.preview.state === 'unsupported') {
    return <Text>该材料{view.preview.reason === 'size' ? '过大' : '类型不支持'}移动端内联预览，请使用下载或分享。</Text>;
  }
  if (view.bytes !== undefined && view.entry.mime.startsWith('image/')) {
    return <Image source={{ uri: `data:${view.entry.mime};base64,${bytesToBase64(view.bytes)}` }} style={{ width: 320, height: 240 }} />;
  }
  return <Text>{view.text}</Text>;
}
