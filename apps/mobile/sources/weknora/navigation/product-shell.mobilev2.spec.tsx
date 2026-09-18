import { describe, expect, it } from 'vitest';
// @ts-expect-error react-test-renderer has no declarations in this workspace
import { create, act } from 'react-test-renderer';
import React from 'react';
import { ProductShell } from './ProductShell.tsx';

/**
 * MX-009 frozen 场景：product-scope-without-happy-sync。
 * 挂载产品壳：四个固定 Tab 可见；挂载期间零网络请求（无 Happy 同步/鉴权拉取）。
 * 观测以 MX009-OBSERVATION JSON 行输出，由 tsx probe 解析。
 */

interface Observation {
  visibleTabs: string[];
  happyAuthRequests: number;
}

function tabLabels(root: ReturnType<typeof create>): string[] {
  return root.root
    .findAll((node: { type: unknown; props: { accessibilityRole?: string } }) => typeof node.type === 'string' && node.props.accessibilityRole === 'tab')
    .map((node: { props: { accessibilityLabel?: string } }) => node.props.accessibilityLabel ?? '');
}

describe('MX-009 product shell', () => {
  it('renders the four fixed tabs and performs zero auth requests at mount', async () => {
    const requests: unknown[] = [];
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (async (...args: unknown[]) => {
      requests.push(args);
      throw new Error('product shell must not issue network requests at mount');
    }) as typeof fetch;
    try {
      let renderer!: ReturnType<typeof create>;
      await act(async () => {
        renderer = create(<ProductShell />);
      });
      const visibleTabs = tabLabels(renderer);
      // Tab 切换仍零请求
      const third = renderer.root.findAll((node: { props: { accessibilityLabel?: string } }) => node.props.accessibilityLabel === '资源')[0];
      await act(async () => {
        third.props.onPress();
      });
      expect(visibleTabs).toEqual(['工作台', '会话', '资源', '我的']);
      expect(requests.length).toBe(0);
      const observation: Observation = { visibleTabs, happyAuthRequests: requests.length };
      console.log(`MX009-OBSERVATION ${JSON.stringify(observation)}`);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
