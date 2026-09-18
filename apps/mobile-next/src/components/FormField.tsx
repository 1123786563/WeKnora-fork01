import React, { useState } from "react";
import { Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon } from "./Icon";

export function FormField({
  label,
  value,
  onChangeText,
  placeholder,
  hint,
  error,
  required = false,
  secure = false,
  autoComplete,
  inputMode,
  multiline = false,
  testID,
  onSubmitEditing,
}: {
  label: string;
  value: string;
  onChangeText: (v: string) => void;
  placeholder?: string;
  hint?: string;
  error?: string;
  required?: boolean;
  secure?: boolean;
  autoComplete?: "email" | "password" | "off";
  inputMode?: "email" | "text" | "numeric" | "decimal";
  multiline?: boolean;
  testID?: string;
  onSubmitEditing?: () => void;
}) {
  const { theme } = useTheme();
  const [focused, setFocused] = useState(false);
  const [reveal, setReveal] = useState(false);

  const styles = StyleSheet.create({
    wrap: { gap: theme.space[6] },
    labelRow: { flexDirection: "row" as const, gap: theme.space[4], alignItems: "center" },
    label: {
      fontSize: theme.type.label.fontSize,
      lineHeight: theme.type.label.lineHeight,
      fontWeight: theme.type.label.fontWeight as "600",
      color: theme.c.ink,
    },
    required: { color: theme.c.danger },
    inputRow: {
      minHeight: 48,
      borderRadius: theme.radius.control,
      borderWidth: 1,
      borderColor: error ? theme.c.danger : focused ? theme.c["control-line"] : theme.c["control-line"],
      backgroundColor: theme.c.surface,
      paddingHorizontal: theme.space[16],
      paddingVertical: multiline ? theme.space[12] : theme.space[8],
      flexDirection: "row" as const,
      alignItems: multiline ? ("flex-start" as const) : ("center" as const),
    },
    input: {
      flex: 1,
      fontSize: theme.type.body.fontSize,
      lineHeight: theme.type.body.lineHeight,
      color: theme.c.ink,
      paddingVertical: 0,
      textAlignVertical: multiline ? "top" : "center",
    },
    meta: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: error ? theme.c.danger : theme.c.muted,
    },
  });

  const secureEntry = secure && !reveal;

  return (
    <View style={styles.wrap} testID={testID}>
      <View style={styles.labelRow}>
        <Text style={styles.label}>{label}</Text>
        {required && (
          <Text style={styles.required} accessibilityLabel="必填">
            *
          </Text>
        )}
      </View>
      <View style={styles.inputRow}>
        <TextInput
          value={value}
          onChangeText={onChangeText}
          placeholder={placeholder}
          placeholderTextColor={theme.c.subtle}
          style={styles.input}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          secureTextEntry={secureEntry}
          autoCapitalize={inputMode === "email" ? "none" : "sentences"}
          keyboardType={inputMode === "email" ? "email-address" : "default"}
          inputMode={inputMode}
          autoComplete={autoComplete}
          multiline={multiline}
          textAlignVertical={multiline ? "top" : "center"}
          accessible
          accessibilityLabel={label}
          onSubmitEditing={onSubmitEditing}
        />
        {secure && (
          <Pressable
            onPress={() => setReveal((v) => !v)}
            accessibilityRole="button"
            accessibilityLabel={reveal ? "隐藏密码" : "显示密码"}
            hitSlop={8}
          >
            <Icon name={reveal ? "moon" : "sun"} color={theme.c.muted} size={18} />
          </Pressable>
        )}
      </View>
      {(error || hint) && (
        <Text style={styles.meta} accessibilityRole={error ? "alert" : undefined}>
          {error ?? hint}
        </Text>
      )}
    </View>
  );
}
