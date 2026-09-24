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
