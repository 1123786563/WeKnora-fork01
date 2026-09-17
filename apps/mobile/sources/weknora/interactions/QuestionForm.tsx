import React, { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { Field } from '../ui/Field.tsx';

/**
 * 问题回复表单（MX-020）：options/schema 服务端校验、长度限制；
 * 提交**用户编辑后的最新文本**（非缓存的初稿）；未知交互只读展示不 fall back。
 */
export interface QuestionFormProps {
  question: string;
  options?: readonly string[];
  maxLength?: number;
  onSubmit: (answer: string) => void;
  onCancel?: () => void;
  testID?: string;
}

export function QuestionForm({ question, options, maxLength = 2000, onSubmit, onCancel, testID }: QuestionFormProps) {
  const { theme } = useWeknoraTheme();
  const [answer, setAnswer] = useState('');
  useEffect(() => {
    // 每次问题切换清空编辑区（不携带上一问的残留）
    setAnswer('');
  }, [question]);
  const over = answer.length > maxLength;
  return (
    <Card testID={testID}>
      <Text accessibilityLabel={`任务执行中的问题：${question}`} style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>
        {question}
      </Text>
      {options && options.length > 0 ? (
        <View style={{ marginTop: theme.spacing[12], gap: theme.spacing[8] }}>
          {options.map((option) => (
            <Button key={option} label={option} variant="secondary" onPress={() => onSubmit(option)} accessibilityLabel={`选择回答：${option}`} size="compact" />
          ))}
        </View>
      ) : null}
      <View style={{ marginTop: theme.spacing[12] }}>
        <Field
          label="你的回答"
          value={answer}
          onChangeText={setAnswer}
          placeholder="输入回复（提交你编辑后的最新文本）"
          multiline
          error={over ? `回答长度超出限制（${maxLength} 字）` : undefined}
          helper={`${answer.length}/${maxLength} 字`}
        />
      </View>
      <View style={{ marginTop: theme.spacing[12], gap: theme.spacing[8] }}>
        <Button
          label="提交回答"
          disabled={answer.trim() === '' || over}
          onPress={() => onSubmit(answer.trim())}
          accessibilityLabel="提交编辑后的回答（recovery 交互 provide_result 语义）"
        />
        {onCancel ? <Button label="返回" variant="secondary" onPress={onCancel} accessibilityLabel="返回不提交" size="compact" /> : null}
      </View>
    </Card>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
void styles;
