// MX-002 probe · 原生依赖基线观察器
// 观察真实依赖解析（node_modules 安装版本 + expo bundledNativeModules 官方矩阵 + RN peer 范围 + 平台工具链），
// 不读 manifest 声明当作事实，不返回构造值。
import { readFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import os from 'node:os';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const mobileDir = path.join(repoRoot, 'apps', 'mobile');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  rendererMatchesReact: boolean;
  nativeBuildPlatforms: string[];
}

// 最小 semver satisfies：支持 exact / ^x.y.z / ~x.y.z（bundledNativeModules 与 RN peer 全部为这三种形态）
function satisfies(version: string, range: string): boolean {
  const exact = range.match(/^[\d.]+$/);
  if (exact) return version === range;
  const m = range.match(/^([\^~])?(\d+)\.(\d+)\.(\d+)$/);
  const v = version.match(/^(\d+)\.(\d+)\.(\d+)/);
  if (!m || !v) return false;
  const [, mod, M1, m1, p1] = m;
  const [, M2, m2, p2] = v;
  if (M1 !== M2) return false;
  if (!mod) return version === range;
  if (mod === '~') return m1 === m2 && Number(p2) >= Number(p1);
  return Number(m2) > Number(m1) || (m2 === m1 && Number(p2) >= Number(p1));
}

async function installedVersion(pkg: string): Promise<string | null> {
  try {
    const raw = JSON.parse(await readFile(path.join(mobileDir, 'node_modules', pkg, 'package.json'), 'utf8')) as { version?: string };
    return raw.version ?? null;
  } catch {
    return null;
  }
}

export async function runProbe(_input: ProbeInput): Promise<Observation> {
  const appPkg = JSON.parse(await readFile(path.join(mobileDir, 'package.json'), 'utf8')) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  const declared = { ...(appPkg.dependencies ?? {}), ...(appPkg.devDependencies ?? {}) };
  const bundled = JSON.parse(
    await readFile(path.join(mobileDir, 'node_modules', 'expo', 'bundledNativeModules.json'), 'utf8'),
  ) as Record<string, string>;

  const mismatches: string[] = [];
  // 1) expo 官方 SDK 矩阵：所有被 bundledNativeModules 覆盖且被 App 声明的依赖，安装版本必须满足期望 range
  for (const [pkg, expected] of Object.entries(bundled)) {
    if (!(pkg in declared)) continue;
    const installed = await installedVersion(pkg);
    if (installed === null) {
      mismatches.push(`${pkg}: declared but not installed`);
      continue;
    }
    if (!satisfies(installed, expected)) mismatches.push(`${pkg}: installed ${installed} !~ ${expected}`);
  }
  // 2) react-dom 与 react 精确一致（renderer 配对）
  const react = await installedVersion('react');
  const reactDom = await installedVersion('react-dom');
  if (react === null || reactDom === null || react !== reactDom) {
    mismatches.push(`react/react-dom pair: ${react} vs ${reactDom}`);
  }
  // 3) react 满足 react-native 声明的 peer 范围
  const rnPkg = JSON.parse(await readFile(path.join(mobileDir, 'node_modules', 'react-native', 'package.json'), 'utf8')) as {
    peerDependencies?: Record<string, string>;
  };
  const rnPeerReact = rnPkg.peerDependencies?.react;
  if (!react || !rnPeerReact || !satisfies(react, rnPeerReact)) {
    mismatches.push(`react ${react} vs react-native peer ${rnPeerReact}`);
  }

  // 平台可构建性：expo 配置声明的平台 ∩ 本机工具链（真实探测，不假设）
  const { default: expoConfig } = (await import(path.join(mobileDir, 'app.config.ts'))) as {
    default: { ios?: unknown; android?: unknown };
  };
  const platforms: string[] = [];
  if (expoConfig.ios) platforms.push('ios');
  if (expoConfig.android) platforms.push('android');
  const buildable: string[] = [];
  for (const platform of platforms) {
    if (platform === 'ios') {
      try {
        await execFileAsync('xcrun', ['-find', 'xcodebuild']);
        buildable.push('ios');
      } catch {
        /* 工具链缺失 → 该平台不可构建 */
      }
    }
    if (platform === 'android') {
      const sdkHome = process.env.ANDROID_HOME ?? path.join(os.homedir(), 'Library', 'Android', 'sdk');
      if (existsSync(path.join(sdkHome, 'build-tools'))) buildable.push('android');
    }
  }

  if (mismatches.length > 0) {
    // 可观测性：不改变 Observation 形状，把真实偏差输出到 stderr 便于诊断
    console.error('[mx-002 probe] dependency mismatches:', mismatches);
  }

  return {
    rendererMatchesReact: mismatches.length === 0,
    // 按配置声明顺序返回（ios 先于 android，与 app.config 声明一致），不做字母序重排
    nativeBuildPlatforms: buildable,
  };
}
