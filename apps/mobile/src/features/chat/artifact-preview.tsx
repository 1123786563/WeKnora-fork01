import { Image, Modal, Pressable, SafeAreaView, ScrollView, Text, View } from 'react-native';
import type { ChatArtifact } from '@weknora/domain/chat/artifacts';
import { classifyNativeArtifactPreview, nativeMarkdownLines } from './artifact-preview.ts';

export { classifyNativeArtifactPreview, nativeMarkdownLines } from './artifact-preview.ts';

export interface NativeArtifactPreviewProps {
  artifact: Pick<ChatArtifact, 'fileName' | 'fileType'>;
  uri?: string;
  content?: string;
  loading?: boolean;
  error?: string;
  labels?: {
    back: string;
    share: string;
    loading: string;
    downloadOnly: string;
  };
  onClose(): void;
  onDownload?(): void;
}

export function NativeArtifactPreview({ artifact, uri, content, loading = false, error, labels, onClose, onDownload }: NativeArtifactPreviewProps) {
  const model = classifyNativeArtifactPreview(artifact);
  const lines = model.kind === 'markdown' && content !== undefined ? nativeMarkdownLines(content) : [];
  const copy = labels ?? { back: 'Back', share: 'Share', loading: 'Loading preview…', downloadOnly: model.label };
  return <Modal visible animationType="slide" onRequestClose={onClose}>
    <SafeAreaView style={{ flex: 1, backgroundColor: '#fff' }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', padding: 14, borderBottomColor: '#eaecf0', borderBottomWidth: 1 }}>
        <Pressable accessibilityRole="button" onPress={onClose}><Text style={{ color: '#2864dc' }}>{copy.back}</Text></Pressable>
        <Text accessibilityRole="header" numberOfLines={1} style={{ flex: 1, marginHorizontal: 12, fontWeight: '700' }}>{artifact.fileName}</Text>
        {onDownload ? <Pressable accessibilityRole="button" onPress={onDownload}><Text style={{ color: '#2864dc' }}>{copy.share}</Text></Pressable> : null}
      </View>
      {loading ? <Text accessibilityRole="progressbar" style={{ padding: 16 }}>{copy.loading}</Text> : null}
      {error ? <Text accessibilityRole="alert" style={{ padding: 16, color: '#b42318' }}>{error}</Text> : null}
      {!loading && !error && model.kind === 'download-only' ? <Text style={{ padding: 16, color: '#667085' }}>{labels?.downloadOnly ?? model.label}</Text> : null}
      {!loading && !error && model.kind === 'image' && uri ? <Image accessibilityLabel={artifact.fileName} source={{ uri }} resizeMode="contain" style={{ flex: 1, width: '100%' }} /> : null}
      {!loading && !error && model.kind === 'text' && content !== undefined ? <ScrollView contentContainerStyle={{ padding: 16 }}><Text selectable style={{ fontFamily: 'monospace', lineHeight: 20 }}>{content}</Text></ScrollView> : null}
      {!loading && !error && model.kind === 'markdown' && content !== undefined ? <ScrollView contentContainerStyle={{ padding: 16 }}>{lines.map((line, index) => <Text key={`${line.kind}:${index}`} selectable style={{ marginBottom: 8, fontSize: line.kind === 'heading' ? 19 : 15, fontWeight: line.kind === 'heading' ? '700' : '400', fontFamily: line.kind === 'code' ? 'monospace' : undefined, color: line.kind === 'bullet' ? '#344054' : '#101828' }}>{line.kind === 'bullet' ? `• ${line.text}` : line.text}</Text>)}</ScrollView> : null}
    </SafeAreaView>
  </Modal>;
}
