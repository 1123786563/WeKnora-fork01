// MX-021 probe · 跨账户通知投递观察器
// frozen 场景：device-A-logout-B-login × late-A-intent。
// 真实 shouldDeliverNotification + RevokeDevicesForOwner（Go 侧）：设备 A 注册→用户 A 登出
// （设备撤销）→用户 B 登录同设备→A 的迟到通知意图不得投递给 B（deliveredToB=0），
// B 的收件箱审批计数为 0；深链恢复顺序（认证→空间→权限）逐级验证。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import {
  parseDeepLink,
  resolveDeepLink,
  shouldDeliverNotification,
} from '../../../apps/mobile/sources/weknora/notifications/deep-link.ts';
import { registerForNotifications } from '../../../apps/mobile/sources/weknora/notifications/registration.ts';

const execFileAsync = promisify(execFile);
import { fileURLToPath } from 'node:url';
import path from 'node:path';
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  deliveredToB: number;
  approvalCount: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'device-A-logout-B-login' || input.fault !== 'late-A-intent') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }

  // 1) 服务端：注册 A 设备→登出撤销→B 登录后 A 通知不投递（真实 Go 测试输出观测）
  const go = await execFileAsync('go', ['test', './internal/handler/session/', '-run', 'TestMX021CrossAccountInbox', '-count=1', '-v'], { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 });
  const goOut = go.stdout + go.stderr;
  if (goOut.includes('FAIL')) throw new Error(`go cross-account test failed: ${goOut.slice(0, 300)}`);
  const marker = goOut.match(/MX021-OBSERVATION (\{[^\n]+\})/);
  if (!marker) throw new Error('MX021-OBSERVATION missing from go test output');
  const server = JSON.parse(marker[1]) as { deliveredToB: number; approvalCount: number };

  // 2) 客户端：A 的迟到意图对 B 会话零投递（真实 shouldDeliverNotification）
  const lateAIntent = { tenant: 't-A', user: 'u-A' };
  const sessionB = { tenantId: 't-B', userId: 'u-B' };
  let deliveredToB = 0;
  if (shouldDeliverNotification(lateAIntent.tenant, lateAIntent.user, sessionB)) deliveredToB += 1;

  // 3) 深链恢复顺序：B 未认证→login；认证但空间不符→switch-space；匹配→open（重查权限）
  const link = parseDeepLink('weknora://interactions/i-9');
  if (!link || link.kind !== 'interaction') throw new Error('deep link parse failed');
  const step1 = resolveDeepLink(link, { authenticated: false, currentTenantId: null }, 't-B');
  if (step1.action !== 'login') throw new Error('unauthenticated deep link must route to login');
  const step2 = resolveDeepLink(link, { authenticated: true, currentTenantId: 't-B' }, 't-A');
  if (step2.action !== 'switch-space') throw new Error('cross-tenant deep link must route to space confirmation');
  const step3 = resolveDeepLink(link, { authenticated: true, currentTenantId: 't-B' }, 't-B');
  if (step3.action !== 'open') throw new Error('matching deep link must open (with permission recheck)');

  // 4) 注册降级：权限拒绝不影响任务（registered=false, degraded）
  const denied = await registerForNotifications(
    { register: async () => { throw new Error('unreachable'); } },
    { id: 'device-1', pushToken: null, platform: 'ios' },
  );
  if (denied.registered || !denied.degraded || denied.reason !== 'permission_denied') {
    throw new Error('permission denial must degrade without blocking tasks');
  }

  return { deliveredToB: server.deliveredToB + deliveredToB, approvalCount: server.approvalCount };
}
