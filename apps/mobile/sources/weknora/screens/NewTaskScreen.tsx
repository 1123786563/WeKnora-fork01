import React, { useMemo, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { Field } from '../ui/Field.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import {
  evaluateSubmitReadiness,
  type NewTaskDraft,
  type SubmitReadiness,
  type TaskAttachmentRef,
} from '@weknora/domain/mobile';

/**
 * 新建任务页 M05（MX-015）：草稿绑定、就绪裁决驱动的提交、Agent 选择入口（M06）、
 * 附件状态展示（未就绪阻塞并说明）；取消保留草稿；离线不自动发送。
 */
export interface NewTaskScreenProps {
  draft: NewTaskDraft;
  agentName?: string;
  onTextChange(text: string): void;
  onBudgetChange(budget: number): void;
  onPickAgent(): void;
  onCancel(): void;
  onSubmit(): void;
  offline?: boolean;
  submitBlockedReason?: SubmitReadiness['reason'];
  testID?: string;
}

const ATTACHMENT_TONE: Record<TaskAttachmentRef['readiness'], BadgeTone> = {
  pending: 'neutral', scanning: 'warning', ready: 'brand', failed: 'danger',
};

const ATTACHMENT_LABEL: Record<TaskAttachmentRef['readiness'], string> = {
  pending: '待处理', scanning: '扫描中', ready: '就绪', failed: '失败',
};

const BLOCK_MESSAGE: Record<NonNullable<SubmitReadiness['reason']>, string> = {
  text_required: '请输入任务内容',
  agent_required: '请选择执行 Agent',
  budget_invalid: '预算上限必须是非负整数',
  attachments_not_ready: '附件尚未就绪（扫描完成后才能提交）',
};

export function NewTaskScreen({ draft, agentName, onTextChange, onBudgetChange, onPickAgent, onCancel, onSubmit, offline, submitBlockedReason, testID }: NewTaskScreenProps) {
  const { theme } = useWeknoraTheme();
  const [budgetText, setBudgetText] = useState(draft.budgetUpper > 0 ? String(draft.budgetUpper) : '');
  const readiness = useMemo(() => evaluateSubmitReadiness(draft), [draft]);
  const blockedReason = submitBlockedReason ?? (readiness.ready ? undefined : readiness.reason);

  return (
    <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[16] }} keyboardShouldPersistTaps="handled">
      {offline ? (
        <StateView kind="offline" message="当前离线" detail="任务草稿会保留，恢复网络后可提交" />
      ) : null}
      <Card>
        <Field label="任务内容" value={draft.text} onChangeText={onTextChange} placeholder="描述要完成的任务" multiline />
        <Pressable accessibilityRole="button" accessibilityLabel={`选择 Agent${agentName ? `，当前 ${agentName}` : '，尚未选择'}`} onPress={onPickAgent} style={[styles.agentRow, { borderColor: theme.colors['control-line'], borderRadius: theme.radius.control, marginTop: theme.spacing[12] }]}>
          <Text style={{ color: draft.agentId ? theme.colors.ink : theme.colors.subtle, flex: 1, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight }}>
            {draft.agentId ? (agentName ?? draft.agentId) : '选择执行 Agent'}
          </Text>
          <StatusBadge tone={draft.agentId ? 'brand' : 'neutral'} label={draft.agentId ? '已选择' : '待选择'} />
        </Pressable>
        <View style={{ marginTop: theme.spacing[12] }}>
          <Field
            label="预算上限"
            value={budgetText}
            onChangeText={(text) => {
              setBudgetText(text);
              const parsed = Number(text);
              onBudgetChange(text.trim() === '' ? 0 : Number.isSafeInteger(parsed) && parsed >= 0 ? parsed : -1);
            }}
            placeholder="0 表示不设上限"
            helper="预算为非负整数；结算以服务端为准"
          />
        </View>
      </Card>

      {draft.attachments.length > 0 ? (
        <Card>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>附件</Text>
          <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
            附件扫描完成后才能提交；扫描中提交会被阻止且草稿保留。
          </Text>
          <View style={{ marginTop: theme.spacing[12], gap: theme.spacing[8] }}>
            {draft.attachments.map((attachment) => (
              <View key={attachment.id} style={styles.row} accessibilityLabel={`附件 ${attachment.name}，${ATTACHMENT_LABEL[attachment.readiness]}${attachment.readiness === 'ready' ? '' : '，阻塞提交'}`}>
                <Text style={{ color: theme.colors.ink, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>{attachment.name}</Text>
                <StatusBadge tone={ATTACHMENT_TONE[attachment.readiness]} label={ATTACHMENT_LABEL[attachment.readiness]} />
              </View>
            ))}
          </View>
        </Card>
      ) : null}

      {blockedReason ? (
        <Text accessibilityRole="alert" style={{ color: theme.colors.warning, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          {BLOCK_MESSAGE[blockedReason]}
        </Text>
      ) : null}

      <View style={{ gap: theme.spacing[12] }}>
        <Button label="创建任务" onPress={onSubmit} disabled={!!blockedReason || offline} accessibilityLabel={blockedReason ? `无法提交：${BLOCK_MESSAGE[blockedReason]}` : '提交任务（先保存请求再发送）'} />
        <Button label="保存草稿并返回" variant="secondary" onPress={onCancel} accessibilityLabel="取消输入并保留可恢复草稿" size="compact" />
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
  agentRow: { flexDirection: 'row', alignItems: 'center', borderWidth: StyleSheet.hairlineWidth, padding: 12 },
});
