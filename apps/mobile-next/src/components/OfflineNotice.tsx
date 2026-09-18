import React from "react";
import { StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon } from "./Icon";
import { useI18n } from "@/i18n";

// 离线横幅（02 规格：当前离线，草稿已保留；允许编辑草稿、禁用危险 mutation 由页面层执行）
export function OfflineNotice({ detail, testID }: { detail?: string; testID?: string }) {
  const { theme } = useTheme();
  const { t } = useI18n();
  const styles = StyleSheet.create({
    wrap: {
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["warning-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    text: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.warning,
    },
  });
  return (
    <View style={styles.wrap} testID={testID ?? "offline-notice"} accessibilityRole="alert">
      <Icon name="wifi" size={16} color={theme.c.warning} />
      <Text style={styles.text}>{detail ?? t("state.offline")}</Text>
    </View>
  );
}
