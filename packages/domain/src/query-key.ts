export interface RequestScope {
  origin: string;
  userId: string | null;
  tenantId: string | null;
  generation: number;
}

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
