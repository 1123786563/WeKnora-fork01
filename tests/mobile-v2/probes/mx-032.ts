// MX-032 probe · 能力清单与证据门禁观察器
// frozen 场景：remote-without-cancel-evidence。
// 真实 Go CapabilityService（经 Go 测试输出观测）+ profile-manifest.json（真实文件读取）：
// remote 缺取消证据 → unavailable(missing_cancel_evidence)；清单与代码门禁一致。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  remoteCapability: string;
  reason: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'remote-without-cancel-evidence') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  // 1) 真实 Go 裁决（含 frozen 断言：missing_cancel_evidence）
  const go = await execFileAsync('go', ['test', './internal/application/service/workbench/', '-run', 'TestMX032ProfileWithMissingEvidence', '-count=1', '-v'], { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 });
  if ((go.stdout + go.stderr).includes('FAIL')) throw new Error('go capability test failed');

  // 2) 部署清单与代码门禁一致性：remote 要求 cancel 证据（无证据即 unavailable）
  const manifest = JSON.parse(await readFile(path.join(repoRoot, 'deploy/mobile-workbench/profile-manifest.json'), 'utf8')) as {
    profiles: Record<string, { deploy_enabled: boolean; evidence_required: string[] }>;
  };
  const remote = manifest.profiles['remote'];
  if (!remote) throw new Error('manifest must define remote profile');
  if (!remote.evidence_required.includes('remote:cancel-evidence')) {
    throw new Error(`manifest must require cancel evidence for remote, got ${JSON.stringify(remote.evidence_required)}`);
  }
  // 部署关闭的 profile 必须直接 deploy_enabled=false（隐藏入口——不置灰）
  for (const hidden of ['personal_node', 'full_happy']) {
    if (manifest.profiles[hidden]?.deploy_enabled !== false) throw new Error(`${hidden} must be deploy-disabled`);
  }
  // core 必须开启
  if (manifest.profiles['core']?.deploy_enabled !== true) throw new Error('core must be deploy-enabled');

  return { remoteCapability: 'unavailable', reason: 'missing_cancel_evidence' };
}
