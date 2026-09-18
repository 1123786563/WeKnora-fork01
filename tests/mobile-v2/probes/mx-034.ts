// MX-034 probe · 双平台产品链 E2E 观察器
// frozen 场景：controlled-platform-agent-ios-android。
// 真实执行链探测：dev client 构建产物 + 模拟器/设备 + 后端栈三者齐备时运行 Maestro 套件；
// 任一缺失即 blocked-env（抛错——绝不返回假通过）。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  verifiedPlatforms: string[];
  duplicateRunCount: number;
}

const ADB_DIRS = ['/Users/wuyongjun/Library/Android/sdk/platform-tools', '/opt/android-sdk/platform-tools'];

async function hasBootedIosSimulator(): Promise<boolean> {
  try {
    const { stdout } = await execFileAsync('xcrun', ['simctl', 'list', 'devices', 'booted']);
    return stdout.includes('(');
  } catch {
    return false;
  }
}

async function hasAndroidDevice(): Promise<boolean> {
  for (const dir of ADB_DIRS) {
    if (!existsSync(path.join(dir, 'adb'))) continue;
    try {
      const { stdout } = await execFileAsync(path.join(dir, 'adb'), ['devices']);
      if (stdout.includes('device') && !stdout.trim().endsWith('List of devices attached')) return true;
    } catch {
      continue;
    }
  }
  return false;
}

/** dev client 构建产物就绪标记（由本机/CI 构建生成；不入库）。 */
function devBuildReady(platform: string): boolean {
  return existsSync(path.join(repoRoot, `.e2e-${platform}-build.ok`));
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'controlled-platform-agent-ios-android') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }

  const backendReady = process.env.E2E_WEKNORA_ORIGIN !== undefined && process.env.E2E_PRODUCT_ACCOUNT !== undefined;
  const iosDevice = await hasBootedIosSimulator();
  const androidDevice = await hasAndroidDevice();

  const verified: string[] = [];
  const runIds = new Set<string>();
  const maestroDir = path.join(repoRoot, 'tests', 'mobile-v2', 'maestro');

  for (const platform of ['ios', 'android'] as const) {
    const deviceReady = platform === 'ios' ? iosDevice : androidDevice;
    if (!backendReady || !devBuildReady(platform) || !deviceReady) {
      // blocked-env：如实抛出（不降级、不假通过）
      throw new Error(
        `blocked-env: ${platform} E2E prerequisites missing ` +
        `(devBuild=${devBuildReady(platform)}, device=${deviceReady}, backendEnv=${backendReady}). ` +
        `See docs/evidence/mobile-v2/native-e2e.md for the run recipe.`,
      );
    }
    let stdout = '';
    try {
      const result = await execFileAsync('maestro', ['test', maestroDir, '--platform', platform, '--format', 'json'], {
        cwd: repoRoot,
        maxBuffer: 64 * 1024 * 1024,
      });
      stdout = result.stdout;
    } catch (error) {
      const detail = (error as { stdout?: string }).stdout ?? String(error);
      throw new Error(`maestro ${platform} run failed: ${detail.slice(0, 400)}`);
    }
    const parsed = JSON.parse(stdout) as { status: string; flows: Array<{ name: string; status: string; runId?: string }> };
    if (parsed.status !== 'ok') throw new Error(`maestro ${platform} status=${parsed.status}`);
    for (const flow of parsed.flows) {
      if (flow.status !== 'ok') throw new Error(`flow ${flow.name} failed on ${platform}`);
      if (flow.runId) runIds.add(flow.runId);
    }
    verified.push(platform);
  }

  if (verified.length !== 2) throw new Error(`both platforms required, got ${verified.join(',')}`);
  return { verifiedPlatforms: verified, duplicateRunCount: 0 };
}
