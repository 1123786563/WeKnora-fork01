import { useLocalSearchParams } from 'expo-router';
import { KnowledgeCollectionScreen } from '@/weknora/knowledge/KnowledgeScreen';

export default function WikiScreen() {
  const { kbId } = useLocalSearchParams<{ kbId: string }>();
  return <KnowledgeCollectionScreen kbId={kbId!} kind="wiki" />;
}
