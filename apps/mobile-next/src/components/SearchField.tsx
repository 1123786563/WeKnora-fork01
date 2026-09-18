import React from "react";
import { StyleSheet, Text, TextInput, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon } from "./Icon";

// 搜索框（M04：防抖由 hook 层做，组件受控）
export function SearchField({
  value,
  onChangeText,
  placeholder = "搜索会话",
  testID,
}: {
  value: string;
  onChangeText: (v: string) => void;
  placeholder?: string;
  testID?: string;
}) {
  const { theme } = useTheme();
  const styles = StyleSheet.create({
    wrap: {
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[8],
      minHeight: 48,
      borderRadius: theme.radius.control,
      backgroundColor: theme.c.surface,
      borderWidth: 1,
      borderColor: theme.c["control-line"],
      paddingHorizontal: theme.space[12],
    },
    input: { flex: 1, fontSize: theme.type.body.fontSize, color: theme.c.ink, paddingVertical: 0 },
  });
  return (
    <View style={styles.wrap} testID={testID ?? "search-field"}>
      <Icon name="search" size={18} color={theme.c.subtle} />
      <TextInput
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
        placeholderTextColor={theme.c.subtle}
        style={styles.input}
        returnKeyType="search"
        accessibilityLabel={placeholder}
      />
    </View>
  );
}

export function SectionHeader({ title, actionLabel, onAction }: { title: string; actionLabel?: string; onAction?: () => void }) {
  const { theme } = useTheme();
  const styles = StyleSheet.create({
    row: {
      flexDirection: "row" as const,
      alignItems: "center",
      justifyContent: "space-between",
      marginBottom: theme.space[12],
    },
    title: {
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    action: { color: theme.c.brand, fontSize: theme.type["body-sm"].fontSize, fontWeight: "600" },
  });
  return (
    <View style={styles.row}>
      <Text style={styles.title}>{title}</Text>
      {actionLabel && onAction && (
        <Text accessibilityRole="button" onPress={onAction} style={styles.action}>
          {actionLabel}
        </Text>
      )}
    </View>
  );
}
