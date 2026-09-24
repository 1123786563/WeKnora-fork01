/** Additive Craft web-artifact facts. These records describe server decisions;
 * parsing them never grants permission or changes a Run. */
export const CRAFT_WEB_CHECK_OUTCOMES = ['passed', 'failed', 'not_run'] as const;
export type CraftWebCheckOutcome = (typeof CRAFT_WEB_CHECK_OUTCOMES)[number];
export const CRAFT_STOP_OUTCOMES = ['requested', 'confirmed', 'unknown'] as const;
export type CraftStopOutcomeStatus = (typeof CRAFT_STOP_OUTCOMES)[number];
export const CRAFT_WRITER_ACQUIRE_OUTCOMES = ['acquired', 'conflict', 'unknown'] as const;
export type CraftWriterAcquireStatus = (typeof CRAFT_WRITER_ACQUIRE_OUTCOMES)[number];
export const CRAFT_DECISIONS = ['approved', 'rejected', 'unknown'] as const;
export type CraftDecisionStatus = (typeof CRAFT_DECISIONS)[number];

export interface CraftInputRecognition { accepted: boolean; understood: boolean; reason: string }
export interface CraftWebCheckEvidence { build: CraftWebCheckOutcome; entry: CraftWebCheckOutcome; preview_reachable: CraftWebCheckOutcome; page_loaded: CraftWebCheckOutcome }
export interface CraftStopOutcome { run_id: string; status: CraftStopOutcomeStatus }
export interface CraftWriterAcquireOutcome { workspace_id: string; status: CraftWriterAcquireStatus }
export interface CraftBudgetPause { run_id: string; reason: string; limit: number; used: number }
export interface CraftRestrictedContribution { version_id: string; evidence_digest: string; restricted: boolean }
export interface CraftShareDecision { version_id: string; evidence_digest: string; owner_id: string; decision: CraftDecisionStatus }
export interface CraftExportFile { path: string; sha256: string; restricted: boolean }
export interface CraftExportManifest { version_id: string; manifest_digest: string; files: CraftExportFile[] }
export interface CraftExportDecision { version_id: string; manifest_digest: string; owner_id: string; decision: CraftDecisionStatus }

function row(value: unknown, label: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid ' + label);
  return value as Record<string, unknown>;
}
function id(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error('invalid ' + field);
  return value;
}
function flag(value: unknown, field: string): boolean {
  if (typeof value !== 'boolean') throw new Error('invalid ' + field);
  return value;
}
function count(value: unknown, field: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new Error('invalid ' + field);
  return value;
}
function choice<T extends string>(value: unknown, choices: readonly T[], field: string): T {
  if (typeof value !== 'string' || !(choices as readonly string[]).includes(value)) throw new Error('invalid ' + field);
  return value as T;
}

export function parseCraftInputRecognition(value: unknown): CraftInputRecognition {
  const v = row(value, 'recognition');
  const accepted = flag(v.accepted, 'accepted');
  const understood = flag(v.understood, 'understood');
  if (understood && !accepted) throw new Error('invalid understood');
  const reason = typeof v.reason === 'string' ? v.reason : '';
  if (accepted && !understood && reason === '') throw new Error('invalid reason');
  return { accepted, understood, reason };
}
export function parseCraftWebCheckEvidence(value: unknown): CraftWebCheckEvidence {
  const v = row(value, 'web evidence');
  return { build: choice(v.build, CRAFT_WEB_CHECK_OUTCOMES, 'build'), entry: choice(v.entry, CRAFT_WEB_CHECK_OUTCOMES, 'entry'), preview_reachable: choice(v.preview_reachable, CRAFT_WEB_CHECK_OUTCOMES, 'preview_reachable'), page_loaded: choice(v.page_loaded, CRAFT_WEB_CHECK_OUTCOMES, 'page_loaded') };
}
export function parseCraftStopOutcome(value: unknown): CraftStopOutcome {
  const v = row(value, 'stop outcome'); return { run_id: id(v.run_id, 'run_id'), status: choice(v.status, CRAFT_STOP_OUTCOMES, 'status') };
}
export function parseCraftWriterAcquireOutcome(value: unknown): CraftWriterAcquireOutcome {
  const v = row(value, 'writer acquisition'); return { workspace_id: id(v.workspace_id, 'workspace_id'), status: choice(v.status, CRAFT_WRITER_ACQUIRE_OUTCOMES, 'status') };
}
export function parseCraftBudgetPause(value: unknown): CraftBudgetPause {
  const v = row(value, 'budget pause'); return { run_id: id(v.run_id, 'run_id'), reason: id(v.reason, 'reason'), limit: count(v.limit, 'limit'), used: count(v.used, 'used') };
}
export function parseCraftRestrictedContribution(value: unknown): CraftRestrictedContribution {
  const v = row(value, 'restricted contribution'); return { version_id: id(v.version_id, 'version_id'), evidence_digest: id(v.evidence_digest, 'evidence_digest'), restricted: flag(v.restricted, 'restricted') };
}
export function parseCraftShareDecision(value: unknown): CraftShareDecision {
  const v = row(value, 'share decision'); return { version_id: id(v.version_id, 'version_id'), evidence_digest: id(v.evidence_digest, 'evidence_digest'), owner_id: id(v.owner_id, 'owner_id'), decision: choice(v.decision, CRAFT_DECISIONS, 'decision') };
}
export function parseCraftExportManifest(value: unknown): CraftExportManifest {
  const v = row(value, 'export manifest');
  if (!Array.isArray(v.files)) throw new Error('invalid files');
  return { version_id: id(v.version_id, 'version_id'), manifest_digest: id(v.manifest_digest, 'manifest_digest'), files: v.files.map((file) => { const f = row(file, 'export file'); return { path: id(f.path, 'path'), sha256: id(f.sha256, 'sha256'), restricted: flag(f.restricted, 'restricted') }; }) };
}
export function parseCraftExportDecision(value: unknown): CraftExportDecision {
  const v = row(value, 'export decision'); return { version_id: id(v.version_id, 'version_id'), manifest_digest: id(v.manifest_digest, 'manifest_digest'), owner_id: id(v.owner_id, 'owner_id'), decision: choice(v.decision, CRAFT_DECISIONS, 'decision') };
}
