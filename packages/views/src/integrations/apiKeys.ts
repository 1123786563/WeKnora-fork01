// API-key table helpers ported from the Vue baseline
// frontend/src/components/ApiIntegrationSettings.vue: stored values are
// masked, full_access/capabilities drive the access-mode label, and a
// freshly created key reveals its value exactly once.

export interface ApiKeyRow {
  id: number | string;
  name: string;
  api_key: string;
  full_access: boolean;
  capabilities?: string[];
  created_at?: string;
}

export function apiKeyValueDisplay(key: ApiKeyRow, reveal: boolean): string {
  if (reveal) return key.api_key;
  const value = key.api_key;
  if (!value) return '(unavailable)';
  if (value.length <= 10) return value;
  return value.slice(0, 5) + '…' + value.slice(-4);
}

export function isFreshKeyVisible(state: { fresh: boolean; hasValue: boolean }): boolean {
  return state.fresh && state.hasValue;
}

export function apiKeyAccessMode(key: ApiKeyRow): string {
  if (key.full_access) return 'Full access';
  const capabilities = (key.capabilities ?? []).filter((item) => item !== '');
  return capabilities.length > 0 ? capabilities.join(', ') : 'Scoped';
}
