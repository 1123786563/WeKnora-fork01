import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { IosEvidenceEntry } from './ios-release-evidence.ts';

const loadMod = () => import('./ios-release-evidence.ts');

// 冒充输入（source 非法值）模拟 JSON.parse 出来的运行时值：编译期字面量被 IosEvidenceSource
// 拒绝正是类型防线的本职，门的运行时防线用 JSON 往返后的宽输入验证——对计划逐字内容的
// 类型层机械偏差，运行时值零变化（先例：release-deps.test.ts 对计划 URL 写法的同款处理）。
const fromJson = (entries: unknown): IosEvidenceEntry[] => JSON.parse(JSON.stringify(entries)) as IosEvidenceEntry[];

test('the acceptance subject lists cover the nine core flows and the four adversity paths verbatim from the issue', async () => {
  const { IOS_ACCEPTANCE_FLOWS, IOS_ADVERSITY_PATHS } = await loadMod();
  assert.deepEqual([...IOS_ACCEPTANCE_FLOWS], [
    'sign-in', 'tenant-switch', 'task', 'background-recovery', 'offline-draft',
    'notification', 'download-share', 'voice-permission', 'secure-storage',
  ]);
  assert.deepEqual([...IOS_ADVERSITY_PATHS], ['weak-network', 'permission-denied', 'cold-start', 'revocation']);
});

test('assembly is total: missing subjects become not-run entries and later entries win', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence({}, [
    { subject: 'sign-in', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/02-login-screen-11s.png' },
    { subject: 'sign-in', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
  ]);
  assert.equal(record.entries.length, 13, '9 flows + 4 adversity paths，缺项自动补齐');
  const signIn = record.entries.find((entry) => entry.subject === 'sign-in')!;
  assert.equal(signIn.source, 'integration-harness', '同 subject 后者覆盖前者');
  assert.equal(record.entries.find((entry) => entry.subject === 'task')?.disposition, 'not-run');
  assert.ok(iosReleaseEvidenceGaps(record).length > 0, '只有一条证据、其余全 not-run 时门必须拦');
});

test('the gate rejects evidence impersonation in all three shapes (AC3: mocks do not count)', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence({}, fromJson([
    { subject: 'sign-in', disposition: 'evidenced', source: 'unit-test', evidence: 'some.test.ts' },
    { subject: 'tenant-switch', disposition: 'evidenced', source: 'integration-harness' },
    { subject: 'task', disposition: 'blocked-env' },
  ]));
  const gaps = iosReleaseEvidenceGaps(record);
  assert.ok(gaps.some((gap) => gap.includes('sign-in') && gap.includes('installed-package or integration-harness')), 'source=unit-test 是冒充');
  assert.ok(gaps.some((gap) => gap.includes('tenant-switch') && gap.includes('evidence pointer')), 'evidenced 必须带证据指针');
  assert.ok(gaps.some((gap) => gap.includes('task') && gap.includes('reason')), 'blocked-env 必须说明缺失的外部资源');
});

test('a complete honest record passes the gate, and blocked-env entries stay listed (not converted to pass)', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence(
    { appPath: 'apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app', builtAt: '2026-09-26T00:00:00.000Z' },
    [
      { subject: 'sign-in', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/02-login-screen-11s.png' },
      { subject: 'tenant-switch', disposition: 'blocked-env', reason: 'single-tenant deployment: the switch path needs a second tenant on the deployment' },
      { subject: 'task', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/task-start-integration-smoke.ts' },
      { subject: 'background-recovery', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
      { subject: 'offline-draft', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/offline-vault-integration-smoke.ts' },
      { subject: 'notification', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/06-push-delivered-unauthorized.png' },
      { subject: 'download-share', disposition: 'blocked-env', reason: 'system share sheet is a true external (spec: real-device acceptance separately)' },
      { subject: 'voice-permission', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/07-after-mic-revoked.png' },
      { subject: 'secure-storage', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
      { subject: 'weak-network', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
      { subject: 'permission-denied', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/07-after-mic-revoked.png' },
      { subject: 'cold-start', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/cold-start.mov' },
      { subject: 'revocation', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/05-deeplink-detail-unauthorized.png' },
    ],
  );
  assert.deepEqual(iosReleaseEvidenceGaps(record), [], '13 个验收项全部 resolved（evidenced 或有理由的 blocked-env）即过门');
  assert.equal(record.entries.find((entry) => entry.subject === 'download-share')?.disposition, 'blocked-env', '门不改写 disposition，blocked-env 不是通过');
});

test('installed-package evidence requires the built artifact path (AC1: no package, no claim)', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence({}, [
    { subject: 'cold-start', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/cold-start.mov' },
  ]);
  assert.ok(iosReleaseEvidenceGaps(record).some((gap) => gap.includes('build.appPath')), '没有 Release 产物就不得主张安装包证据');
});

test('buildRecordFromOutcomes propagates gate violations so the CLI exits non-zero', async () => {
  // 裁决（mimosa-adjudications.md 裁决 3，ruling via escalation）：CLI 的 argv 读取与
  // 路径校验逻辑移入 src/ 模块直测；scripts/emit-acceptance-record.ts 仅留薄壳 import main。
  const { buildRecordFromOutcomes } = await import('./ios-release-evidence-cli.ts');
  const failing = buildRecordFromOutcomes({ build: {}, entries: fromJson([{ subject: 'sign-in', disposition: 'evidenced', source: 'prototype', evidence: 'x' }]) });
  assert.ok(failing.gaps.length > 0, 'source=prototype + 其余 not-run 都是门违规');
  const assembled = buildRecordFromOutcomes({
    build: { appPath: 'apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app' },
    entries: [{ subject: 'sign-in', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/02-login-screen-11s.png' }],
  });
  assert.equal(assembled.record.entries.length, 13, '装配仍是 total 的，缺失项以 not-run 呈现给门');
});
