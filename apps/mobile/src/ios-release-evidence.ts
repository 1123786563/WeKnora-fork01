/** T39（Issue #69）验收证据契约：九大核心工作流 × 四条逆境路径的处置分类与「不冒充」门。
 *  AC3 原文：「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实
 *  集成证据。」本模块只认两类证据来源：installed-package（Release 安装包在模拟器实跑的
 *  截图/录屏/日志）与 integration-harness（opt-in 真实部署的最高稳定 Interface 集成证据）。
 *  blocked-env 是如实登记、不是通过（#71 口径：「任何 skipped 或 blocked-env 不计为通过」）。 */

export const IOS_ACCEPTANCE_FLOWS = [
  'sign-in', 'tenant-switch', 'task', 'background-recovery', 'offline-draft',
  'notification', 'download-share', 'voice-permission', 'secure-storage',
] as const;
export type IosAcceptanceFlow = (typeof IOS_ACCEPTANCE_FLOWS)[number];

export const IOS_ADVERSITY_PATHS = ['weak-network', 'permission-denied', 'cold-start', 'revocation'] as const;
export type IosAdversityPath = (typeof IOS_ADVERSITY_PATHS)[number];

export type IosEvidenceSubject = IosAcceptanceFlow | IosAdversityPath;
export type IosEvidenceSource = 'installed-package' | 'integration-harness';
export type IosEvidenceDisposition = 'evidenced' | 'blocked-env' | 'not-run';

export interface IosEvidenceEntry {
  subject: IosEvidenceSubject;
  disposition: IosEvidenceDisposition;
  /** disposition='evidenced' 时必填：证据文件或证据键（相对 docs/plans/issue30-sweep/ 的路径或 smoke 证据键）。 */
  evidence?: string;
  /** disposition!=='evidenced' 时必填：blocked-env 必须点名缺失的外部资源。 */
  reason?: string;
  source?: IosEvidenceSource;
}

export interface IosReleaseBuildFacts {
  /** Release-iphonesimulator/WeKnora.app 的仓库相对路径；未构建时缺省。 */
  appPath?: string;
  builtAt?: string;
  buildLog?: string;
}

export interface IosReleaseEvidenceRecord {
  build: IosReleaseBuildFacts;
  entries: IosEvidenceEntry[];
  generatedAt: string;
}

const REQUIRED_SUBJECTS: readonly IosEvidenceSubject[] = [...IOS_ACCEPTANCE_FLOWS, ...IOS_ADVERSITY_PATHS];

function normalizeEntry(entry: IosEvidenceEntry): IosEvidenceEntry {
  if (entry.disposition === 'evidenced') {
    return { ...entry, source: entry.source ?? 'installed-package', reason: undefined };
  }
  // 非 evidenced 条目不携带证据指针与来源：它们不是通过，别让残留字段制造「已验证」错觉。
  // reason 原样透传（不兜底）：缺理由的 blocked-env 必须被 iosReleaseEvidenceGaps 的
  // 「entries must carry a reason」分支点名——若在这里兜底默认文案，该分支永不触发，
  // Review Focus 5 的门语义就失效了。
  return { subject: entry.subject, disposition: entry.disposition, reason: entry.reason };
}

/** total 化装配：按 subject 去重（后者覆盖前者），缺失 subject 自动补 not-run 条目；绝不 reject。 */
export function assembleIosReleaseEvidence(build: IosReleaseBuildFacts, entries: IosEvidenceEntry[]): IosReleaseEvidenceRecord {
  const bySubject = new Map<IosEvidenceSubject, IosEvidenceEntry>();
  for (const entry of entries) bySubject.set(entry.subject, normalizeEntry(entry));
  for (const subject of REQUIRED_SUBJECTS) {
    if (!bySubject.has(subject)) bySubject.set(subject, { subject, disposition: 'not-run', reason: 'no entry provided' });
  }
  return { build, entries: REQUIRED_SUBJECTS.map((subject) => bySubject.get(subject)!), generatedAt: new Date().toISOString() };
}

/** AC 门（机器检查）：返回违规列表，空数组 = 记录形态可接受。三条语义：
 *  ①not-run 恒为违规（未解决的验收项）；②blocked-env 带理由即形态合法——它是如实登记、
 *  不是通过，「blocked-env 未清零不得宣布 Issue 通过」是发布结论层的义务（#71），
 *  门故意不改写 disposition；③evidenced 必须来自两类真实来源且携带证据指针。 */
export function iosReleaseEvidenceGaps(record: IosReleaseEvidenceRecord): string[] {
  const gaps: string[] = [];
  const subjects = new Set(record.entries.map((entry) => entry.subject));
  for (const subject of REQUIRED_SUBJECTS) {
    if (!subjects.has(subject)) gaps.push(`missing acceptance subject: ${subject}`);
  }
  for (const entry of record.entries) {
    if (entry.disposition === 'evidenced') {
      if (entry.source !== 'installed-package' && entry.source !== 'integration-harness') {
        gaps.push(`${entry.subject}: evidenced entries must come from installed-package or integration-harness (AC3: mocks do not count)`);
      }
      if (entry.evidence === undefined || entry.evidence.trim() === '') {
        gaps.push(`${entry.subject}: evidenced entries must carry an evidence pointer`);
      }
    } else if (entry.disposition === 'not-run') {
      gaps.push(`${entry.subject}: not-run entries are unresolved acceptance subjects`);
    } else if (entry.reason === undefined || entry.reason.trim() === '') {
      gaps.push(`${entry.subject}: ${entry.disposition} entries must carry a reason`);
    }
  }
  const claimsPackage = record.entries.some((entry) => entry.disposition === 'evidenced' && entry.source === 'installed-package');
  if (claimsPackage && (record.build.appPath === undefined || record.build.appPath.trim() === '')) {
    gaps.push('installed-package evidence requires build.appPath of the Release artifact');
  }
  return gaps;
}
