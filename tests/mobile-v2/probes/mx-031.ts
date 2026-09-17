// MX-031 probe · 用量三桶与最终性观察器
// frozen 场景：reserved120-pending38-settled1260。
// 真实 presentUsage：三桶原值透传、pending 未最终、paymentRequests 结构性 0（只读视图）。
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { presentUsage, type TenantUsageSnapshot } from '../../../apps/mobile/sources/weknora/commercial/usage-presenter.ts';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  reserved: number;
  pending: number;
  settled: number;
  paymentRequests: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'reserved120-pending38-settled1260') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  const snapshot: TenantUsageSnapshot = {
    tenantId: 't1',
    billingAccountScope: 'billing-account-1',
    reserved: { value: 120, currency: '点', unitLabel: '点' },
    pending: { value: 38, currency: '点', unitLabel: '点' },
    settled: { value: 1260, currency: '点', unitLabel: '点' },
    breakdown: {
      byokModelFees: { value: 240, currency: '点', unitLabel: '点' },
      platformServiceFees: { value: 1020, currency: '点', unitLabel: '点' },
    },
    asOf: '2026-09-18T08:00:00Z',
    viewerRole: 'owner',
  };

  const presentation = presentUsage(snapshot);
  if (presentation.reserved !== 120 || presentation.pending !== 38 || presentation.settled !== 1260) {
    throw new Error(`three buckets must pass through verbatim, got ${presentation.reserved}/${presentation.pending}/${presentation.settled}`);
  }
  if (presentation.pendingIsFinal) throw new Error('pending must never be presented as final');
  if (!presentation.asOfLabel.includes('2026-09-18T08:00:00Z')) throw new Error('as_of must be visible');
  if (!presentation.scopeLabel.includes('不与其他空间混用')) throw new Error('billing scope isolation must be stated');
  if (presentation.paymentRequests !== 0) throw new Error('read-only view must never issue payment requests');

  // 源级：UsageScreen 无购买/退款/支付入口
  const screen = await readFile(path.join(repoRoot, 'apps/mobile/sources/weknora/screens/UsageScreen.tsx'), 'utf8');
  for (const banned of ['purchase', 'PaymentRequest', 'buy(', 'refund(', '钱包']) {
    if (screen.toLowerCase().includes(banned.toLowerCase())) throw new Error(`usage screen must not contain payment entry: ${banned}`);
  }

  return {
    reserved: presentation.reserved,
    pending: presentation.pending,
    settled: presentation.settled,
    paymentRequests: presentation.paymentRequests,
  };
}
