import type { SkillStatus } from '@weknora/api-client';

const secretKeys = new Set(['apikey', 'appsecret', 'accesstoken', 'refreshtoken', 'token', 'clientsecret', 'password', 'secret']);

function isSecretKey(key: string): boolean {
  return secretKeys.has(key.replace(/[^a-zA-Z0-9]/g, '').toLowerCase());
}

export function normalizeSandboxConfigIds(value: string): string[] {
  return [...new Set(value.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean))];
}

export function isSafeSkillFilePath(value: string): boolean {
  const path = value.trim();
  if (!path || path.startsWith('/') || path.includes('\\')) return false;
  return path.split('/').every((segment) => segment !== '' && segment !== '.' && segment !== '..');
}

export function shouldPollInstalledSkill(status: SkillStatus): boolean {
  return status === 'installing' || status === 'removing';
}

export function installProgress(statuses: SkillStatus[]): { total: number; complete: number; active: number; failed: number } {
  const active = statuses.filter(shouldPollInstalledSkill).length;
  const failed = statuses.filter((status) => status === 'failed').length;
  return { total: statuses.length, complete: statuses.length - active, active, failed };
}

export function redactDebugValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(redactDebugValue);
  if (value === null || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value as Record<string, unknown>).map(([key, item]) => [key, isSecretKey(key) ? '[redacted]' : redactDebugValue(item)]));
}
