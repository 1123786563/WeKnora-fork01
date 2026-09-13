import type { DataSource } from '@weknora/api-client';

import type { DataSourceResource } from '@weknora/api-client';

export type DataSourceWorkspaceRole = 'owner' | 'admin' | 'contributor' | 'viewer' | string | undefined;

/** Data-source mutations are restricted to the same workspace roles as the API. */
export function canManageDataSources(role: DataSourceWorkspaceRole): boolean {
  return role === 'owner' || role === 'admin';
}

export type ResourceCheckState = 'checked' | 'indeterminate' | 'unchecked';

function descendants(resources: DataSourceResource[], id: string): string[] {
  const children = resources.filter((resource) => resource.parent_id === id);
  return children.flatMap((child) => [child.external_id, ...descendants(resources, child.external_id)]);
}

function ancestors(resources: DataSourceResource[], id: string): string[] {
  const parent = resources.find((resource) => resource.external_id === id)?.parent_id;
  return parent ? [parent, ...ancestors(resources, parent)] : [];
}

export function resourceCheckState(resources: DataSourceResource[], selectedIds: string[], id: string): ResourceCheckState {
  const selected = new Set(selectedIds);
  if (selected.has(id) || ancestors(resources, id).some((ancestor) => selected.has(ancestor))) return 'checked';
  return descendants(resources, id).some((descendant) => selected.has(descendant)) ? 'indeterminate' : 'unchecked';
}

/** Keeps selected resource IDs as a minimal cover set, matching the Vue tree. */
export function toggleDataSourceResourceSelection(resources: DataSourceResource[], selectedIds: string[], id: string): string[] {
  const cover = new Set(selectedIds);
  const state = resourceCheckState(resources, selectedIds, id);
  const childIds = new Set(descendants(resources, id));
  if (state === 'unchecked') {
    for (const selected of cover) if (childIds.has(selected)) cover.delete(selected);
    cover.add(id);
    return [...cover];
  }
  const chain = [id, ...ancestors(resources, id)];
  const covering = chain.find((ancestor) => cover.has(ancestor));
  if (covering && covering !== id) {
    cover.delete(covering);
    const path = [id, ...ancestors(resources, id)];
    const index = path.indexOf(covering);
    for (const branch of resources.filter((resource) => resource.parent_id === covering)) {
      if (branch.external_id !== path[index - 1]) cover.add(branch.external_id);
    }
  } else cover.delete(id);
  for (const selected of cover) if (childIds.has(selected)) cover.delete(selected);
  return [...cover];
}

export function dataSourceStatusLabel(source: Pick<DataSource, 'status'>): string {
  return typeof source.status === 'string' && source.status.trim() ? source.status : 'unknown';
}

export function safeDataSourceType(source: Pick<DataSource, 'type'>): string {
  return typeof source.type === 'string' && source.type.trim() ? source.type : 'unknown';
}
