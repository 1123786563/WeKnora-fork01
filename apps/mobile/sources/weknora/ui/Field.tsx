import React, { useId, useState } from 'react';
import { StyleSheet, Text, TextInput, View } from 'react-native';
import { useWeknoraTheme } from './theme.ts';

/**
 * 表单字段（MX-008）：标签/输入/说明/错误一体，props 驱动。
 * - 状态文字与颜色同时存在（错误不只靠描红）；
 * - 动态说明（字数等）拼接完整句子供读屏；
 * - 不固定输入高度（大字体不裁切）；secure 条目读屏不朗读内容。
 */
export interface FieldProps {
  label: string;
  value: string;
  onChangeText: (text: string) => void;
  placeholder?: string;
  helper?: string;
  error?: string;
  multiline?: boolean;
  secureTextEntry?: boolean;
  /** 可访问名定制（默认 label） */
  accessibilityLabel?: string;
  testID?: string;
}

export function Field({ label, value, onChangeText, placeholder, helper, error, multiline = false, secureTextEntry = false, accessibilityLabel, testID }: FieldProps) {
  const { theme } = useWeknoraTheme();
  const [focused, setFocused] = useState(false);
  const describedBy = `${label}-helper`;
  return (
    <View style={styles.container}>
      <Text style={{ color: theme.colors.ink, fontSize: theme.typography.label.fontSize, lineHeight: theme.typography.label.lineHeight, fontWeight: '600' }}>
        {label}
      </Text>
      <TextInput
        testID={testID}
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
        placeholderTextColor={theme.colors.subtle}
        multiline={multiline}
        secureTextEntry={secureTextEntry}
        accessible
        accessibilityLabel={error ? `${accessibilityLabel ?? label}，${error}` : accessibilityLabel ?? label}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        style={[
          styles.input,
          {
            color: theme.colors.ink,
            borderColor: error ? theme.colors.danger : focused ? theme.colors.focus : theme.colors['control-line'],
            borderRadius: theme.radius.control,
            paddingHorizontal: theme.spacing[12],
            paddingVertical: theme.spacing[12],
            fontSize: theme.typography.body.fontSize,
            backgroundColor: theme.colors.surface,
          },
        ]}
      />
      {error ? (
        <Text accessibilityLabel={`错误。${error}`} style={{ color: theme.colors.danger, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          {error}
        </Text>
      ) : helper ? (
        <Text nativeID={describedBy} style={{ color: theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          {helper}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { gap: 6, alignSelf: 'stretch' },
  input: { borderWidth: StyleSheet.hairlineWidth, minHeight: 48, textAlignVertical: 'top' },
});
