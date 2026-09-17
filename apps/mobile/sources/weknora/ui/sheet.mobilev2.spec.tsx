import { describe, expect, it } from 'vitest';
// @ts-expect-error react-test-renderer has no declarations in this workspace
import { create, act } from 'react-test-renderer';
import React from 'react';
import { Sheet } from './Sheet.tsx';
import { Button } from './Button.tsx';
import { focusCalls, backHandlers } from '../../../tests/mobile-v2/rn-mock.tsx';

/**
 * MX-008 frozen 场景：mounted-native-sheet × dismiss-before-decision。
 * 真实组件挂载（react-test-renderer + rn-mock 基底）：
 * - 决定前关闭（遮罩/返回键）不产生任何决定调用；
 * - 关闭后读屏焦点回到触发元素。
 * 观测以 MX008-OBSERVATION JSON 行输出，由 tests/mobile-v2/probes/mx-008.ts 解析。
 */

interface Observation {
  decisionCount: number;
  focusTarget: string;
}

function runScenario(): Observation {
  focusCalls.length = 0;
  backHandlers.length = 0;
  let decisionCount = 0;
  let visible = true;
  const triggerRef: React.RefObject<unknown> = { current: { __testTag: 'trigger' } };

  let renderer!: ReturnType<typeof create>;
  act(() => {
    renderer = create(
      <Sheet
        visible={visible}
        title="确认操作"
        triggerFocusRef={triggerRef}
        onClose={() => { visible = false; }}
        children={<text>{'确认内容'}</text>}
        footer={
          <Button
            label="批准"
            accessibilityLabel="批准该工具调用"
            onPress={() => { decisionCount += 1; }}
          />
        }
      />,
    );
  });

  // 决定按钮存在但未被触发；先关闭（遮罩语义：accessibilityLabel「关闭」的 Pressable）
  const closeTarget = renderer.root.findAll((node: { props: { accessibilityLabel?: string } }) => node.props.accessibilityLabel === '关闭')[0];
  if (!closeTarget) throw new Error('close affordance not found');
  act(() => {
    closeTarget.props.onPress();
  });

  // 焦点调用记录中应包含 trigger 标签（打开时的 title 焦点 + 关闭后的 trigger 回归）
  const focusTarget = focusCalls.includes('trigger') ? 'trigger' : String(focusCalls.at(-1) ?? 'none');
  return { decisionCount, focusTarget };
}

describe('MX-008 sheet', () => {
  it('dismiss before decision: no decision, focus returns to trigger', () => {
    const observed = runScenario();
    // 逐项断言（vitest 层），同时输出结构化观测给 tsx probe
    expect(observed.decisionCount).toBe(0);
    expect(observed.focusTarget).toBe('trigger');
    console.log(`MX008-OBSERVATION ${JSON.stringify(observed)}`);
  });

  it('busy button does not dispatch repeatedly', () => {
    let dispatched = 0;
    let renderer!: ReturnType<typeof create>;
    act(() => {
      renderer = create(<Button label="提交" loading onPress={() => { dispatched += 1; }} />);
    });
    const button = renderer.root.findByProps({ accessibilityLabel: '提交' });
    act(() => { button.props.onPress(); });
    act(() => { button.props.onPress(); });
    expect(dispatched).toBe(0);
    console.log(`MX008-BUSY-OBSERVATION ${JSON.stringify({ dispatched })}`);
  });
});
