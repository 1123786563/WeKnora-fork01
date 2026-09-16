import type { WikiGraphNode } from '@weknora/api-client';

export interface GraphNodePosition {
  slug: string;
  title: string;
  left: number;
  top: number;
  tone: 'primary' | 'muted';
}

/** Deterministic, dependency-free layout for the native graph preview. */
export function graphNodePositions(nodes: readonly WikiGraphNode[], center?: string): GraphNodePosition[] {
  const columns = 2;
  return nodes.slice(0, 40).map((node, index) => ({
    slug: node.slug,
    title: node.title,
    left: (index % columns) * 50,
    top: Math.floor(index / columns) * 76,
    tone: node.slug === center ? 'primary' : 'muted',
  }));
}
