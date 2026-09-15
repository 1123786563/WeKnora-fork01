import { useLocalSearchParams } from 'expo-router';
import { KnowledgeCollectionScreen } from '@/weknora/knowledge/KnowledgeScreen';

export default function FaqScreen() {
  const { kbId } = useLocalSearchParams<{ kbId: string }>();
  return <KnowledgeCollectionScreen kbId={kbId!} kind="faq" />;
}
