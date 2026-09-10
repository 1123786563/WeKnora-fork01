export interface CapabilityDescriptor {
  supported: boolean;
  reason?: string;
}

export type CapabilityMap = Record<string, CapabilityDescriptor>;

export function normalizeCapabilityMap(value: Record<string, unknown> | undefined): CapabilityMap {
  const result: CapabilityMap = {};
  for (const [key, raw] of Object.entries(value ?? {})) {
    if (typeof raw === 'boolean') {
      result[key] = { supported: raw };
      continue;
    }
    if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) continue;
    const supported = (raw as Record<string, unknown>).supported;
    if (typeof supported !== 'boolean') continue;
    const reason = (raw as Record<string, unknown>).reason;
    result[key] = {
      supported,
      ...(typeof reason === 'string' && reason.trim() ? { reason } : {}),
    };
  }
  return result;
}

export function isCapabilitySupported(
  capabilities: CapabilityMap,
  key: string | undefined,
  options: { liteMode?: boolean; edition?: string } = {},
): boolean {
  if (!key) return true;
  if (key === 'organizations' && (options.liteMode === true || options.edition?.trim().toLowerCase() === 'lite')) return false;
  if (key === 'settings.sandbox.docker') return capabilities[key]?.supported === true;
  return capabilities[key]?.supported !== false;
}
