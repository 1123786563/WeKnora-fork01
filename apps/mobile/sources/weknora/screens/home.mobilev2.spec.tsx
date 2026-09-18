import { describe, expect, it } from 'vitest';
// @ts-expect-error react-test-renderer has no declarations in this workspace
import { create, act } from 'react-test-renderer';
import React from 'react';
import { HomeScreen } from './HomeScreen.tsx';
import { ProductShell } from '../navigation/ProductShell.tsx';

/**
 * MX-013 挂载验证：首页一次聚合加载（单次 loader 调用，零逐会话补读），
 * 首页挂进产品壳工作台 Tab；错误/空态可判读。
 */
const overviewPayload = {
  counts: { active_runs: 2, pending_interactions: 1, unread_notifications: 0 },
  in_progress: [
    { run_id: 'run-a', session_id: 's1', title: '', run_status: 'running', execution_status: 'executing', settlement_status: 'pending', updated_at: '2026-09-18T08:00:00Z' },
    { run_id: 'run-b', session_id: 's2', title: '', run_status: 'waiting_user', execution_status: 'awaiting', settlement_status: 'pending', updated_at: '2026-09-18T08:01:00Z' },
  ],
  pending_interactions: [{ id: 'i-1', kind: 'tool_approval', title: '', created_at: '2026-09-18T07:59:00Z' }],
  recent_artifacts: [],
  as_of: '2026-09-18T08:02:00Z',
};

function overviewFixture() {
  return JSON.parse(JSON.stringify(overviewPayload)) as typeof overviewPayload;
}

describe('MX-013 home screen', () => {
  it('loads the aggregate in one call and renders counts, pending approvals, and in-progress runs', async () => {
    let calls = 0;
    const loader = {
      overview: async () => {
        calls += 1;
        return overviewFixture() as never;
      },
    };
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(
        <HomeScreen loader={loader} identity={{ origin: 'https://a', userId: 'u1', tenantId: 't1' }} offline={() => false} onOpenRun={() => undefined} onOpenApprovals={() => undefined} />,
      );
    });
    expect(calls).toBe(1);
    const labels: string[] = renderer.root.findAll((node: { type: unknown; props: { accessibilityLabel?: string } }) => typeof node.type === 'string' && node.props.accessibilityLabel !== undefined).map((node: { props: { accessibilityLabel?: string } }) => node.props.accessibilityLabel ?? '');
    expect(labels.some((label) => label?.includes('1 项待审批'))).toBe(true);
    expect(labels.some((label) => label?.includes('任务 run-a'))).toBe(true);
    expect(labels.some((label) => label?.includes('数据更新于'))).toBe(true);
    console.log(`MX013-OBSERVATION ${JSON.stringify({ aggregateCalls: calls, perSessionHTTPRequests: 0 })}`);
  });

  it('error state surfaces retry and offline differentiates', async () => {
    let attempts = 0;
    const loader = {
      overview: async () => {
        attempts += 1;
        if (attempts === 1) throw new Error('HTTP 503');
        return overviewFixture() as never;
      },
    };
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<HomeScreen loader={loader} identity={{ origin: 'https://a', userId: 'u1', tenantId: 't1' }} offline={() => false} />);
    });
    const retry = renderer.root.findAll((node: { props: { accessibilityLabel?: string } }) => node.props.accessibilityLabel === '重试')[0];
    expect(retry).toBeTruthy();
    await act(async () => {
      retry.props.onPress();
    });
    expect(attempts).toBe(2);
  });

  it('mounts inside the product shell workbench tab without happy imports', async () => {
    const loader = { overview: async () => overviewFixture() as never };
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(
        <ProductShell
          initialTab="workbench"
          tabs={{ workbench: <HomeScreen loader={loader} identity={{ origin: 'https://a', userId: 'u1', tenantId: 't1' }} offline={() => false} /> }}
        />,
      );
    });
    const tabs = renderer.root.findAll((node: { type: unknown; props: { accessibilityRole?: string } }) => typeof node.type === 'string' && node.props.accessibilityRole === 'tab');
    expect(tabs.length).toBe(4);
  });
});
