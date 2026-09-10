import type { KnowledgeFolderNode, KnowledgeFolderTree } from '@weknora/contracts';

export interface FlatKnowledgeFolder {
  path: string;
  name: string;
  document_count: number;
  total_count: number;
  depth: number;
}

export function flattenKnowledgeFolders(tree: KnowledgeFolderTree): FlatKnowledgeFolder[] {
  const result: FlatKnowledgeFolder[] = [{
    path: '', name: 'Root', document_count: tree.root_document_count, total_count: tree.total_document_count, depth: 0,
  }];
  function visit(nodes: KnowledgeFolderNode[], depth: number): void {
    for (const node of nodes) {
      result.push({ path: node.path, name: node.name, document_count: node.document_count, total_count: node.total_count, depth });
      if (node.children?.length) visit(node.children, depth + 1);
    }
  }
  visit(tree.folders, 0);
  return result;
}
