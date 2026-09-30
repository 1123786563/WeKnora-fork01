/** T39 验收记录发射器的 src 实现：读观察结果 JSON → 装配记录 → AC 门校验 → 打印记录。
 *  CLI 入口是薄壳 apps/mobile/scripts/emit-acceptance-record.ts（用法：
 *  pnpm exec tsx apps/mobile/scripts/emit-acceptance-record.ts <outcomes.json>）。
 *  outcomes.json 形如 { "build": { "appPath": "…", "builtAt": "…", "buildLog": "…" },
 *                       "entries": [ { "subject": "sign-in", "disposition": "…", … } ] }
 *  门不通过时打印违规清单并以非零码退出——阻塞验收结论，而不是放行。
 *
 *  布局来源（mimosa-adjudications.md 裁决 3，ruling via escalation）：Mimosa 路径穿越规则
 *  对 scripts/ 下 argv-readFileSync 形态一票拦截且不识别任何校验逻辑（形态级误判），裁决
 *  批准把读取与校验实现移入 src/ 直测，scripts/ 仅留薄壳。路径边界三件套在此完整保留。 */
import { readFileSync } from 'node:fs';
import { isAbsolute, resolve, sep } from 'node:path';
import {
  assembleIosReleaseEvidence,
  iosReleaseEvidenceGaps,
  type IosEvidenceEntry,
  type IosReleaseBuildFacts,
  type IosReleaseEvidenceRecord,
} from './ios-release-evidence.ts';

export interface AcceptanceOutcomes {
  build: IosReleaseBuildFacts;
  entries: IosEvidenceEntry[];
}

export function buildRecordFromOutcomes(outcomes: AcceptanceOutcomes): { record: IosReleaseEvidenceRecord; gaps: string[] } {
  const record = assembleIosReleaseEvidence(outcomes.build ?? {}, outcomes.entries ?? []);
  return { record, gaps: iosReleaseEvidenceGaps(record) };
}

/** CLI 主体（由薄壳以 process.argv.slice(2) 调用）。Mimosa 路径穿越门三件套：
 *  ①禁止含 ../ 或 .. 段（及 Windows 分隔符）的输入；②resolve 规范化；
 *  ③必须落在当前仓库根（允许目录）之内。三关全过才允许 readFileSync。 */
export function main(argv: string[]): void {
  const arg = argv[0] ?? '';
  const repoRoot = process.cwd();
  const requested = isAbsolute(arg) ? resolve(arg) : resolve(repoRoot, arg);
  if (
    arg === '' || arg.includes('..') || arg.includes('\\') ||
    requested === repoRoot || !requested.startsWith(repoRoot + sep)
  ) {
    console.error(`outcomes file must be a ../-free relative path inside the repository root (${repoRoot}), got: ${arg === '' ? '<missing>' : arg}`);
    process.exit(1);
  }
  const raw = JSON.parse(readFileSync(requested, 'utf8')) as AcceptanceOutcomes;
  const { record, gaps } = buildRecordFromOutcomes(raw);
  if (gaps.length > 0) {
    console.error(JSON.stringify({ gaps }, null, 2));
    process.exit(1);
  }
  console.log(JSON.stringify(record, null, 2));
}
