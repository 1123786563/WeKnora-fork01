import type { DataSource } from '@weknora/api-client';

export type DataSourceWorkspaceRole = 'owner' | 'admin' | 'contributor' | 'viewer' | string | undefined;

/** Data-source mutations are restricted to the same workspace roles as the API. */
export function canManageDataSources(role: DataSourceWorkspaceRole): boolean {
  return role === 'owner' || role === 'admin';
}

export function dataSourceStatusLabel(source: Pick<DataSource, 'status'>): string {
  return typeof source.status === 'string' && source.status.trim() ? source.status : 'unknown';
}

export function safeDataSourceType(source: Pick<DataSource, 'type'>): string {
  return typeof source.type === 'string' && source.type.trim() ? source.type : 'unknown';
}
