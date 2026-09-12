import type { RequestScope } from './scope.ts';
export { isCapabilitySupported, normalizeCapabilityMap } from './access/capability.ts';
export type { CapabilityDescriptor, CapabilityMap } from './access/capability.ts';
export { canDuplicateKBCard, canManageKBCard, filterKnowledgeBases, groupKnowledgeBaseSections, isKnowledgeBaseInitialized, isSharedKbEditable, mergeAllScopeKnowledgeBases } from './knowledge/list.ts';
export type { KnowledgeBaseSection } from './knowledge/list.ts';
export type {
  KnowledgeBaseCreatorFilter,
  KnowledgeBaseListFilter,
  KnowledgeBaseListResult,
  MergedKnowledgeBase,
  MergedOwnedKnowledgeBase,
  MergedSharedKnowledgeBase,
  OwnedKnowledgeBase,
  SharedKnowledgeBaseLike,
  KnowledgeBaseViewer,
} from './knowledge/list.ts';

export * from './settings/local-preferences.ts';
export type { RequestScope } from './scope.ts';
export { createScopeController } from './scope.ts';
export type { ScopeController, ScopeHandle, ScopeInput } from './scope.ts';

export type QueryKey = readonly [
  'weknora',
  string,
  string | null,
  string | null,
  string,
  unknown,
];

export function scopedKey(scope: RequestScope, resource: string, params: unknown = {}): QueryKey {
  if (!resource.trim()) throw new Error('resource must not be empty');
  return [
    'weknora',
    scope.origin.replace(/\/+$/, ''),
    scope.userId,
    scope.tenantId,
    resource,
    params,
  ];
}