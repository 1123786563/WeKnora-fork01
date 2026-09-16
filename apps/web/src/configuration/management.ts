import type { SandboxConfigRecord, SkillCatalog, SkillCatalogInstallation, SkillFile, SkillStatus } from '@weknora/api-client';

const secretKeys = new Set(['apikey', 'appsecret', 'accesstoken', 'refreshtoken', 'token', 'clientsecret', 'password', 'secret']);

function isSecretKey(key: string): boolean {
  return secretKeys.has(key.replace(/[^a-zA-Z0-9]/g, '').toLowerCase());
}

export function normalizeSandboxConfigIds(value: string): string[] {
  return [...new Set(value.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean))];
}

export interface SkillFileTreeRow {
  path: string;
  name: string;
  depth: number;
  isDir: boolean;
  size?: number;
}

export function skillFileTree(files: readonly SkillFile[]): SkillFileTreeRow[] {
  const rows = new Map<string, SkillFileTreeRow>();
  for (const file of files) {
    const segments = file.path.split('/');
    for (let index = 0; index < segments.length; index += 1) {
      const path = segments.slice(0, index + 1).join('/');
      if (rows.has(path)) continue;
      rows.set(path, {
        path,
        name: segments[index]!,
        depth: index,
        isDir: index < segments.length - 1,
        ...(index === segments.length - 1 ? { size: file.size } : {}),
      });
    }
  }
  return [...rows.values()].sort((left, right) => left.path < right.path ? -1 : left.path > right.path ? 1 : 0);
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

// --- Skill settings parity helpers, ported from frontend/src/views/settings/SkillSettings.vue,
// frontend/src/components/SandboxSkillsPanel.vue, and frontend/src/components/SkillFilesPanel.vue ---

/** Sandbox backends managed as named workspace configurations (frontend/src/api/system/index.ts:918). */
export const NAMED_SANDBOX_BACKEND_TYPES = ['cube', 'e2b', 'docker'] as const;

export function isNamedSandboxBackend(type: string): boolean {
  return (NAMED_SANDBOX_BACKEND_TYPES as readonly string[]).includes(type);
}

/** Catalog card copy collapses whitespace so long descriptions stay single-line (SkillSettings.vue:640). */
export function compactSkillText(value: string): string {
  return value.replace(/\s+/g, ' ').trim();
}

function positiveMegabytes(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

/** Mirrors MAX_SKILL_BUNDLE_SIZE_MB: runtime config > default 256, clamped to [knowledge cap, 512] (frontend/src/utils/index.ts:23-42). */
export function maxSkillBundleMB(runtime?: { MAX_SKILL_BUNDLE_SIZE_MB?: unknown; MAX_FILE_SIZE_MB?: unknown }): number {
  const config = runtime ?? (typeof window === 'undefined' ? undefined : (window as unknown as { __RUNTIME_CONFIG__?: { MAX_SKILL_BUNDLE_SIZE_MB?: unknown; MAX_FILE_SIZE_MB?: unknown } }).__RUNTIME_CONFIG__);
  const fileCap = positiveMegabytes(config?.MAX_FILE_SIZE_MB, 50);
  return Math.min(512, Math.max(positiveMegabytes(config?.MAX_SKILL_BUNDLE_SIZE_MB, 256), fileCap));
}

export function isZipBundle(file: { name: string; type?: string }): boolean {
  return file.name.toLowerCase().endsWith('.zip') || file.type === 'application/zip';
}

export type SkillRegisterErrorKind = 'bundleTooLarge' | 'bundleTooManyFiles' | 'bundleTooManyZipEntries';

/** Maps server register failures onto the Vue copy buckets (SkillSettings.vue:958-979). */
export function classifySkillRegisterError(raw: string): { kind: SkillRegisterErrorKind; count?: string } | null {
  if (/cannot exceed \d+\s*MB/i.test(raw)) return { kind: 'bundleTooLarge' };
  const tooManyFiles = raw.match(/skill directory holds more than (\d+) files/i);
  if (tooManyFiles) return { kind: 'bundleTooManyFiles', count: tooManyFiles[1] };
  const tooManyEntries = raw.match(/archive has more than (\d+) zip entries/i);
  if (tooManyEntries) return { kind: 'bundleTooManyZipEntries', count: tooManyEntries[1] };
  const legacyTooMany = raw.match(/archive holds more than (\d+) files/i);
  if (legacyTooMany) return { kind: 'bundleTooManyFiles', count: legacyTooMany[1] };
  return null;
}

/** Live installations are every row the sandbox has not removed yet (SkillSettings.vue:488). */
export function liveCatalogInstalls(item: SkillCatalog): SkillCatalogInstallation[] {
  return (item.installations ?? []).filter((installation) => installation.status && installation.status !== 'removed');
}

/** A catalog entry can only be deleted while no sandbox still holds it (SkillSettings.vue:492). */
export function canDeleteCatalog(item: SkillCatalog): boolean {
  return liveCatalogInstalls(item).length === 0;
}

export function isInstallBusy(installation: SkillCatalogInstallation): boolean {
  return installation.status === 'installing' || installation.status === 'removing';
}

export function installOutdated(item: SkillCatalog, installation: SkillCatalogInstallation): boolean {
  return Boolean(item.bundleSha256 && installation.bundleSha256 && item.bundleSha256 !== installation.bundleSha256);
}

/** failed first, then busy, stale, ready-but-off, ready (SkillSettings.vue:680). */
export function installPriority(item: SkillCatalog, installation: SkillCatalogInstallation): number {
  if (installation.status === 'failed') return 0;
  if (isInstallBusy(installation)) return 1;
  if (installOutdated(item, installation)) return 2;
  if (installation.status === 'ready' && !installation.enabled) return 3;
  return 4;
}

export type InstallEntryTone = 'failed' | 'busy' | 'off' | 'stale' | 'ready';

/** Chip and panel-row tone mirrors the Vue entry class map (SkillSettings.vue:775). */
export function installEntryTone(item: SkillCatalog, installation: SkillCatalogInstallation): InstallEntryTone {
  if (installation.status === 'failed') return 'failed';
  if (isInstallBusy(installation)) return 'busy';
  if (installation.status === 'ready' && !installation.enabled) return 'off';
  if (installOutdated(item, installation)) return 'stale';
  return 'ready';
}

export function installName(installation: SkillCatalogInstallation): string {
  return installation.sandboxConfigName || installation.sandboxConfigId;
}

/** Sandboxes that could still receive this skill, name-sorted (SkillSettings.vue:688). */
export function unusedInstallTargets(item: SkillCatalog, namedConfigs: readonly SandboxConfigRecord[]): SandboxConfigRecord[] {
  const live = new Set(liveCatalogInstalls(item).map((installation) => installation.sandboxConfigId));
  return namedConfigs
    .filter((config) => !live.has(config.id))
    .sort((left, right) => left.name.localeCompare(right.name, undefined, { sensitivity: 'base' }));
}

export interface SkillInstallsView { installs: SkillCatalogInstallation[]; available: SandboxConfigRecord[]; canAdd: boolean; needsPanel: boolean }

/** Priority-sorted installs plus the remaining targets (SkillSettings.vue:695). */
export function installsView(item: SkillCatalog, namedConfigs: readonly SandboxConfigRecord[]): SkillInstallsView {
  const installs = [...liveCatalogInstalls(item)].sort((left, right) => {
    const diff = installPriority(item, left) - installPriority(item, right);
    return diff !== 0 ? diff : installName(left).localeCompare(installName(right), undefined, { sensitivity: 'base' });
  });
  const available = unusedInstallTargets(item, namedConfigs);
  return { installs, available, canAdd: available.length > 0, needsPanel: installs.length + available.length > 1 };
}

export interface SandboxPickRow { config: SandboxConfigRecord; install?: SkillCatalogInstallation; selectable: boolean; busy: boolean; ready: boolean }

/** Checkbox rows for the add/install drawers (SkillSettings.vue:547). */
export function sandboxPickRows(item: SkillCatalog | null, namedConfigs: readonly SandboxConfigRecord[], mode: 'remaining' | 'all', sessionIds: readonly string[]): SandboxPickRow[] {
  const byId = new Map((item ? liveCatalogInstalls(item) : []).map((installation) => [installation.sandboxConfigId, installation]));
  const session = new Set(sessionIds);
  return namedConfigs
    .filter((config) => {
      if (mode === 'all') return true;
      const installation = byId.get(config.id);
      if (session.has(config.id)) return true;
      if (installation && isInstallBusy(installation)) return true;
      if (!installation || installation.status === 'failed') return true;
      return false;
    })
    .map((config) => {
      const installation = byId.get(config.id);
      const busy = Boolean(installation && isInstallBusy(installation));
      const ready = installation?.status === 'ready';
      return { config, ...(installation === undefined ? {} : { install: installation }), selectable: !busy && !ready, busy, ready };
    });
}

/** docker shows its image; cube/e2b show the API host (SkillSettings.vue:513). */
export function sandboxTargetLine(record: SandboxConfigRecord): string {
  if (record.sandbox_type === 'docker') return record.config?.docker?.image?.trim() || '';
  const remote = record.config?.e2b || record.config?.cube;
  const raw = remote?.api_url?.trim() || '';
  if (!raw) return '';
  try { return new URL(raw).host; } catch { return raw; }
}

export function backendLabelKey(type: string): string {
  return `settings.sandbox.backends.${type}`;
}

/** Workspace env-var helpers ported from frontend/src/views/settings/envVarState.ts:17-75. */
export const MAX_ENV_VALUE_BYTES = 8192;

export function isValidEnvValueLength(value: string): boolean {
  return new TextEncoder().encode(value).length <= MAX_ENV_VALUE_BYTES;
}

export function editedSkillEnvPayload(declaredNames: readonly string[], drafts: Record<string, string> | undefined): Record<string, string> {
  if (!drafts) return {};
  const declared = new Set(declaredNames);
  return Object.fromEntries(Object.entries(drafts).filter(([name]) => declared.has(name)));
}

export function adminSkillEnvClearPayload(name: string): Record<string, string> {
  return { [name]: '' };
}

export function canClearAdminSkillEnv(env: { isSet?: boolean }): boolean {
  return env.isSet === true;
}

export function clearSubmittedSkillEnvDrafts(current: Record<string, string>, submitted: Record<string, string>): Record<string, string> {
  const remaining = { ...current };
  for (const [name, value] of Object.entries(submitted)) if (remaining[name] === value) delete remaining[name];
  return remaining;
}

/** Script verification reports one finding per line; each line is readable on its own (SandboxSkillsPanel.vue:1071). */
export function installErrorLines(error: string | undefined): string[] {
  return (error ?? '').split('\n').map((line) => line.trim()).filter(Boolean);
}

/** File browser tree: SKILL.md pinned first, directories before files, locale order (SkillFilesPanel.vue:332). */
export interface SkillFileNode { name: string; path: string; isDir: boolean; children?: SkillFileNode[] }

export function buildSkillFileTree(files: readonly SkillFile[]): SkillFileNode[] {
  const root: SkillFileNode = { name: '', path: '', isDir: true, children: [] };
  for (const entry of files) {
    const parts = entry.path.split('/').filter(Boolean);
    let current = root;
    parts.forEach((part, index) => {
      const isLast = index === parts.length - 1;
      const nodePath = parts.slice(0, index + 1).join('/');
      if (!current.children) current.children = [];
      let next = current.children.find((child) => child.name === part);
      if (!next) {
        next = { name: part, path: nodePath, isDir: !isLast, ...(isLast ? {} : { children: [] }) };
        current.children.push(next);
      } else if (!isLast) {
        next.isDir = true;
        if (!next.children) next.children = [];
      }
      current = next;
    });
  }
  const sortNodes = (list: SkillFileNode[]) => {
    list.sort((left, right) => {
      if (left.path === 'SKILL.md') return -1;
      if (right.path === 'SKILL.md') return 1;
      if (left.isDir !== right.isDir) return left.isDir ? -1 : 1;
      return left.name.localeCompare(right.name);
    });
    for (const node of list) if (node.children) sortNodes(node.children);
  };
  sortNodes(root.children ?? []);
  return root.children ?? [];
}

export function collectSkillDirPaths(nodes: readonly SkillFileNode[]): string[] {
  const out: string[] = [];
  const walk = (list: readonly SkillFileNode[]) => {
    for (const node of list) {
      if (!node.isDir) continue;
      out.push(node.path);
      if (node.children) walk(node.children);
    }
  };
  walk(nodes);
  return out;
}

export interface SkillFileRow { name: string; path: string; isDir: boolean; depth: number }

export function flattenSkillFileRows(nodes: readonly SkillFileNode[], expanded: ReadonlySet<string>): SkillFileRow[] {
  const out: SkillFileRow[] = [];
  const walk = (list: readonly SkillFileNode[], depth: number) => {
    for (const node of list) {
      out.push({ name: node.name, path: node.path, isDir: node.isDir, depth });
      if (node.isDir && expanded.has(node.path) && node.children?.length) walk(node.children, depth + 1);
    }
  };
  walk(nodes, 0);
  return out;
}

/** SKILL.md frontmatter renders as a definition list (SkillFilesPanel.vue:427). */
export interface SkillFrontmatterField { key: string; value: string; code: boolean }

export function formatFrontmatterValue(raw: string): { value: string; code: boolean } {
  let value = raw.trim();
  if ((value.startsWith('"') && value.endsWith('"') && value.length >= 2) || (value.startsWith("'") && value.endsWith("'") && value.length >= 2)) value = value.slice(1, -1);
  if ((value.startsWith('{') && value.endsWith('}')) || (value.startsWith('[') && value.endsWith(']'))) {
    try { return { value: JSON.stringify(JSON.parse(value), null, 2), code: true }; } catch { return { value, code: true }; }
  }
  return { value, code: false };
}

export function parseFrontmatterFields(raw: string): SkillFrontmatterField[] {
  const fields: SkillFrontmatterField[] = [];
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const index = trimmed.indexOf(':');
    if (index <= 0) continue;
    const key = trimmed.slice(0, index).trim();
    if (!/^[A-Za-z0-9_-]+$/.test(key)) continue;
    fields.push({ key, ...formatFrontmatterValue(trimmed.slice(index + 1)) });
  }
  return fields;
}

export function splitMarkdownFrontmatter(text: string): { fields: SkillFrontmatterField[]; body: string } {
  const source = text.replace(/^\uFEFF/, '');
  const match = source.match(/^(?:[ \t]*\r?\n)*---[ \t]*\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/);
  if (!match) return { fields: [], body: source };
  return { fields: parseFrontmatterFields(match[1] ?? ''), body: source.slice(match[0].length) };
}

export function isMarkdownPath(filePath: string): boolean {
  const base = filePath.split('/').pop() ?? filePath;
  const index = base.lastIndexOf('.');
  const extension = index < 0 ? '' : base.slice(index + 1).toLowerCase();
  return extension === 'md' || extension === 'markdown';
}
