import type { DataSource } from '@weknora/api-client';

export function dataSourceStatusLabel(source: Pick<DataSource, 'status'>): string {
  return typeof source.status === 'string' && source.status.trim() ? source.status : 'unknown';
}

export function safeDataSourceType(source: Pick<DataSource, 'type'>): string {
  return typeof source.type === 'string' && source.type.trim() ? source.type : 'unknown';
}
