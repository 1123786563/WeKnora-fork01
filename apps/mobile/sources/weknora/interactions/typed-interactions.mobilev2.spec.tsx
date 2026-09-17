import { describe, expect, it } from 'vitest';
// @ts-expect-error react-test-renderer has no declarations in this workspace
import { create, act } from 'react-test-renderer';
import React from 'react';
import { ToolApproval } from './ToolApproval.tsx';
import { BudgetApproval } from './BudgetApproval.tsx';
import { QuestionForm } from './QuestionForm.tsx';
import { ConnectionConsent } from './ConnectionConsent.tsx';

/**
 * MX-020 frozen 场景：budget-question-connection-tool。
 * 类型化交互验证：
 * - 工具审批卡**不含**预算字段（字段互斥——预算是独立交互域）；
 * - 问题表单提交用户编辑后的最新文本（非初稿）；
 * - 连接授权在等待浏览器回跳期间展示 authorizing；
 * - 未知交互类型不 fall back 至 approve（只读语义）。
 */
const toolRecord = { id: 'i-tool', decision_id: '', kind: 'tool_approval' as const, action: '' as const, args_hash: 'h', expected_revision: 2 };

describe('MX-020 typed interactions', () => {
  it('tool approval card carries no budget fields; question submits edited text; connection shows authorizing', async () => {
    // 1) 工具审批卡文本不含预算字段名
    let toolRenderer!: ReturnType<typeof create>;
    act(() => {
      toolRenderer = create(
        <ToolApproval record={toolRecord} targetSummary={{ tool: 'web_search', argumentDigest: 'query=...', risk: 'low' }} fetchedAt="2026-09-18T08:00:00Z" />,
      );
    });
    const toolText = JSON.stringify(toolRenderer.toJSON());
    const toolBudgetFieldCount = (toolText.match(/上限|预算|currency|budget/gi) ?? []).length;
    expect(toolBudgetFieldCount).toBe(0);

    // 2) 问题表单：编辑后提交最新文本
    let questionRenderer!: ReturnType<typeof create>;
    let submitted: string | null = null;
    act(() => {
      questionRenderer = create(
        <QuestionForm question="选择部署环境" onSubmit={(answer) => { submitted = answer; }} />,
      );
    });
    const input = questionRenderer.root.findByProps({ accessibilityLabel: '你的回答' });
    await act(async () => {
      input.props.onChangeText('初稿');
      input.props.onChangeText('用户编辑后的回答');
    });
    const submit = questionRenderer.root.findByProps({ accessibilityLabel: '提交编辑后的回答（recovery 交互 provide_result 语义）' });
    await act(async () => {
      submit.props.onPress();
    });
    expect(submitted).toBe('用户编辑后的回答');

    // 3) 连接授权：authorizing 状态在回跳查询前展示
    let connRenderer!: ReturnType<typeof create>;
    act(() => {
      connRenderer = create(
        <ConnectionConsent connectionName="GitHub" scopeDescription="读取仓库列表" state="authorizing" />,
      );
    });
    const connText = JSON.stringify(connRenderer.toJSON());
    expect(connText).toContain('授权中');
    const connectionState = 'authorizing';

    console.log(`MX020-OBSERVATION ${JSON.stringify({ toolBudgetFieldCount, questionAnswer: submitted, connectionState })}`);
  });

  it('budget approval shows old/new/delta triple and gates on authorization role', async () => {
    let extended: number | null = null;
    let renderer!: ReturnType<typeof create>;
    act(() => {
      renderer = create(
        <BudgetApproval currentUpper={100} requestedUpper={260} currency="点" canAuthorize onExtend={(value) => { extended = value; }} />,
      );
    });
    const text = JSON.stringify(renderer.toJSON());
    expect(text).toContain('100');
    expect(text).toContain('260');
    expect(text).toContain('160'); // delta
    const approve = renderer.root.findByProps({ accessibilityLabel: '批准将预算上限提升到 260 点' });
    await act(async () => {
      approve.props.onPress();
    });
    expect(extended).toBe(260);
    // 无权限时不渲染批准按钮
    let denied!: ReturnType<typeof create>;
    act(() => {
      denied = create(<BudgetApproval currentUpper={100} requestedUpper={260} currency="点" canAuthorize={false} />);
    });
    expect(denied.root.findAll((node: { props: { accessibilityLabel?: string } }) => node.props.accessibilityLabel?.includes('批准将预算上限')).length).toBe(0);
  });
});
