import type { DataSourceResource } from '@weknora/api-client';

export type ResourceCheckState = 'checked' | 'indeterminate' | 'unchecked';

function descendants(resources: readonly DataSourceResource[], id: string): Set<string> {
  const result = new Set<string>();
  let frontier = [id];
  while (frontier.length > 0) {
    const next: string[] = [];
    for (const resource of resources) {
      if (resource.parent_id && frontier.includes(resource.parent_id) && !result.has(resource.external_id)) {
        result.add(resource.external_id);
        next.push(resource.external_id);
      }
    }
    frontier = next;
  }
  return result;
}

export function resourceCheckStates(resources: readonly DataSourceResource[], selectedIds: readonly string[]): Map<string, ResourceCheckState> {
  const selected = new Set(selectedIds);
  const states = new Map<string, ResourceCheckState>();
  const children = new Map<string, DataSourceResource[]>();
  for (const resource of resources) if (resource.parent_id) children.set(resource.parent_id, [...(children.get(resource.parent_id) ?? []), resource]);
  function walk(resource: DataSourceResource, ancestorChecked: boolean): boolean {
    const checked = ancestorChecked || selected.has(resource.external_id);
    let childSelected = false;
    for (const child of children.get(resource.external_id) ?? []) if (walk(child, checked)) childSelected = true;
    states.set(resource.external_id, checked ? 'checked' : childSelected ? 'indeterminate' : 'unchecked');
    return checked || childSelected;
  }
  for (const resource of resources) if (!resource.parent_id) walk(resource, false);
  for (const resource of resources) if (!states.has(resource.external_id)) states.set(resource.external_id, selected.has(resource.external_id) ? 'checked' : 'unchecked');
  return states;
}

export function toggleResourceSelection(resources: readonly DataSourceResource[], selectedIds: readonly string[], id: string): string[] {
  const next = new Set(selectedIds);
  const state = resourceCheckStates(resources, selectedIds).get(id) ?? 'unchecked';
  const childIds = descendants(resources, id);
  if (state === 'unchecked') {
    const parentById = new Map(resources.map((resource) => [resource.external_id, resource.parent_id]));
    let covered = false;
    for (let current: string | undefined = id; current; current = parentById.get(current)) if (next.has(current)) covered = true;
    if (covered) return [...next];
    next.add(id);
    for (const childId of childIds) next.delete(childId);
  } else {
    const parentById = new Map(resources.map((resource) => [resource.external_id, resource.parent_id]));
    const chain: string[] = [id];
    for (let current = parentById.get(id); current; current = parentById.get(current)) chain.push(current);
    let coveredAt = -1;
    for (let index = chain.length - 1; index >= 0; index--) if (next.has(chain[index]!)) { coveredAt = index; break; }
    if (coveredAt > 0) {
      next.delete(chain[coveredAt]!);
      for (let index = coveredAt; index > 0; index--) {
        const parentId = chain[index]!;
        const nextId = chain[index - 1]!;
        for (const sibling of resources) if (sibling.parent_id === parentId && sibling.external_id !== nextId) next.add(sibling.external_id);
      }
    }
    next.delete(id);
    for (const childId of childIds) next.delete(childId);
  }
  return [...next];
}
