import { useLocalSearchParams } from 'expo-router';
import { KnowledgeDocumentScreen } from '@/weknora/knowledge/KnowledgeScreen';

export default function DocumentScreen() {
  const { documentId } = useLocalSearchParams<{ documentId: string }>();
  return <KnowledgeDocumentScreen documentId={documentId!} />;
}
