import type { ScopeLease } from '../runtime/types.ts';
import type { DiffHunk } from './diff.ts';

/** Task Material 深模块（module-seams §7）类型合同。Task 5/6 补全 Port 与句柄类型。 */

export type MaterialEntryKind = 'artifact' | 'diff' | 'test-report';

export type PreviewVerdict =
  | { state: 'supported' }
  | { state: 'unsupported'; reason: 'mime' | 'size' };

export interface EvidenceCitation {
  seq: number;
  occurredAt: string;
  type: string;
  source?: string;
  detail: string;
}

export interface MaterialEntry {
  materialId: string;
  index: number;
  kind: MaterialEntryKind;
  name: string;
  mime: string;
  size: number;
  version: string;
  sourceRun: string;
  createdAt?: string;
  digest?: string;
}

export interface MaterialIndex {
  runId: string;
  materials: MaterialEntry[];
  terminal: { available: boolean };
}

export type MaterialRef =
  | { kind: MaterialEntryKind; runId: string; materialId: string }
  | { kind: 'evidence'; runId: string }
  | { kind: 'terminal'; runId: string; cursor?: number };

export interface TerminalLine {
  seq: number;
  occurredAt: string;
  stream: 'stdout' | 'stderr';
  text: string;
}

export type MaterialView =
  | { kind: 'artifact' | 'test-report'; entry: MaterialEntry; preview: PreviewVerdict; text?: string; bytes?: Uint8Array }
  | { kind: 'diff'; entry: MaterialEntry; preview: PreviewVerdict; hunks: DiffHunk[]; malformed: boolean; raw?: string }
  | { kind: 'evidence'; citations: EvidenceCitation[] }
  | { kind: 'terminal'; lines: TerminalLine[]; nextCursor?: number; readOnly: true };

export type MaterialIntent =
  | { kind: 'download'; runId: string; materialId: string }
  | { kind: 'share'; runId: string; materialId: string }
  | { kind: 'terminal-input'; text: string };

export type MaterialActResult =
  | { kind: 'grant'; materialId: string; url: string; expiresAt: string }
  | { kind: 'shared'; materialId: string };

export interface MaterialEvent {
  type: 'grant-expired' | 'scope-closed';
  materialId?: string;
  reason?: string;
}

export interface TaskMaterialHandle {
  index(input: { runId: string }): Promise<MaterialIndex>;
  open(ref: MaterialRef): Promise<MaterialView>;
  act(intent: MaterialIntent): Promise<MaterialActResult>;
  subscribe(listener: (event: MaterialEvent) => void): () => void;
  close(reason: string): void;
}

export interface TaskMaterial {
  open(input: { lease: ScopeLease }): TaskMaterialHandle;
}
