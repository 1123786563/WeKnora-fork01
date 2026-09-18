// MX-001 probe · 仓库文件锁覆盖观察器
// 观察真实磁盘上的 task-index 与 file-ownership 注册表，不做任何业务假设。
// 注册表缺失/不完整时返回未覆盖缺口，不抛错伪造成功。
import { readFile } from 'node:fs/promises';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const execFileAsync = promisify(execFile);

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const taskIndexPath = path.join(repoRoot, 'docs', 'design', 'mobile-v2', 'plans', 'task-index.json');
const registryPath = path.join(repoRoot, 'docs', 'evidence', 'mobile-v2', 'file-ownership.json');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  unownedGaps: string[];
  duplicateOwners: string[];
}

interface RegistryEntry {
  owners?: string[];
  shared_serialized?: boolean;
  [key: string]: unknown;
}

interface Registry {
  files?: Record<string, RegistryEntry>;
}

export async function runProbe(_input: ProbeInput): Promise<Observation> {
  const rawIndex = await readFile(taskIndexPath, 'utf8');
  const index = JSON.parse(rawIndex) as { tasks?: Array<{ id: string; file_lock_keys?: string[]; writes?: string[] }> };

  let registry: Registry = {};
  try {
    registry = JSON.parse(await readFile(registryPath, 'utf8')) as Registry;
  } catch {
    registry = {};
  }

  // task-index 声明的全部写锁及其声明者
  const claimants = new Map<string, Set<string>>();
  for (const task of index.tasks ?? []) {
    const locks = new Set([...(task.file_lock_keys ?? []), ...(task.writes ?? [])]);
    for (const file of locks) {
      const set = claimants.get(file) ?? new Set<string>();
      set.add(task.id);
      claimants.set(file, set);
    }
  }

  const files = registry.files ?? {};
  const unownedGaps = new Set<string>();
  const duplicateOwners = new Set<string>();

  for (const [file, declared] of [...claimants.entries()].sort(([a], [b]) => a.localeCompare(b))) {
    const entry = files[file];
    if (!entry || !Array.isArray(entry.owners) || entry.owners.length === 0) {
      unownedGaps.add(file);
      continue;
    }
    const registered = new Set(entry.owners);
    let covers = true;
    for (const owner of declared) {
      if (!registered.has(owner)) covers = false;
    }
    if (!covers) {
      // 注册表未覆盖该文件的全部任务声明者 → 视为未完成预登记
      unownedGaps.add(file);
      continue;
    }
    if (declared.size > 1 && entry.shared_serialized !== true) {
      duplicateOwners.add(file);
    }
  }

  // 验收守护：注册表 kind 必须与基线 HEAD 的 git 树一致（对照 baseline ls-tree，而非生成时磁盘快照）
  const baseline = typeof (registry as { baseline_head?: unknown }).baseline_head === 'string'
    ? (registry as { baseline_head?: string }).baseline_head
    : undefined;
  if (baseline) {
    const { stdout } = await execFileAsync('git', ['ls-tree', '-r', '--name-only', baseline], { cwd: repoRoot, maxBuffer: 64 * 1024 * 1024 });
    const trackedAtBaseline = new Set(stdout.split('\n').filter(Boolean));
    for (const [file, entry] of Object.entries(files)) {
      if (typeof entry.kind !== 'string') continue;
      const expected = trackedAtBaseline.has(file) ? 'modify' : 'create';
      if (entry.kind !== expected) {
        throw new Error(`kind conflict: ${file} registry=${entry.kind} baseline=${expected}`);
      }
    }
  }

  return {
    unownedGaps: [...unownedGaps].sort(),
    duplicateOwners: [...duplicateOwners].sort(),
  };
}
