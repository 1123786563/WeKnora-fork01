import { useLocalSearchParams } from 'expo-router';
import { KnowledgeDetailScreen } from '@/weknora/knowledge/KnowledgeScreen';

export default function KnowledgeBaseScreen() {
  const { kbId } = useLocalSearchParams<{ kbId: string }>();
  return <KnowledgeDetailScreen kbId={kbId!} />;
}
