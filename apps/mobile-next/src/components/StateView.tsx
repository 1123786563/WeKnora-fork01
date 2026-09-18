import React from "react";
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "./Icon";
import { useI18n } from "@/i18n";

// 通用页面状态（02 规格固定文案；默认文案集中在 i18n，props 可覆盖）。loading 骨架无假业务计数；
// error 保留重试；forbidden 不展示敏感正文；empty 只描述当前 scope。
export type ViewState =
  | { kind: "loading" }
  | { kind: "empty"; title?: string; description?: string }
  | { kind: "error"; retry?: () => void; message?: string }
  | { kind: "forbidden" }
  | { kind: "unknown" };

export function StateView({ state, testID }: { state: ViewState; testID?: string }) {
  const { theme } = useTheme();
  const { t } = useI18n();
  const styles = StyleSheet.create({
    wrap: {
      padding: theme.space[24],
      alignItems: "center",
      gap: theme.space[12],
    },
    title: {
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
      textAlign: "center",
    },
    desc: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
      textAlign: "center",
    },
    retry: {
      marginTop: theme.space[8],
      minHeight: 44,
      paddingHorizontal: theme.space[20],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center" as const,
      justifyContent: "center" as const,
    },
    retryText: { color: theme.c.brand, fontWeight: "600", fontSize: theme.type.label.fontSize },
  });

  if (state.kind === "loading") {
    return (
      <View style={styles.wrap} testID={testID ?? "state-loading"} accessibilityLabel={t("state.loading")}>
        <ActivityIndicator size="small" color={theme.c.brand} />
        <Text style={styles.desc}>{t("state.loading")}</Text>
      </View>
    );
  }
  if (state.kind === "forbidden") {
    return (
      <View style={styles.wrap} testID={testID ?? "state-forbidden"}>
        <Icon name="lock" size={24} color={theme.c.subtle} />
        <Text style={styles.title}>{t("state.forbidden")}</Text>
      </View>
    );
  }
  if (state.kind === "unknown") {
    return (
      <View style={styles.wrap} testID={testID ?? "state-unknown"}>
        <Icon name="refresh" size={24} color={theme.c.subtle} />
        <Text style={styles.title}>{t("state.unknown")}</Text>
        <Text style={styles.desc}>{t("state.uncertain")}</Text>
      </View>
    );
  }
  if (state.kind === "error") {
    return (
      <View style={styles.wrap} testID={testID ?? "state-error"}>
        <Icon name="alert" size={24} color={theme.c.danger} />
        <Text style={styles.title}>{state.message ?? t("state.error")}</Text>
        {state.retry && (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={t("action.retry")}
            style={styles.retry}
            onPress={state.retry}
          >
            <Text style={styles.retryText}>{t("action.retry")}</Text>
          </Pressable>
        )}
      </View>
    );
  }
  return (
    <View style={styles.wrap} testID={testID ?? "state-empty"}>
      <Icon name="spark" size={24} color={theme.c.subtle} />
      <Text style={styles.title}>{state.title ?? t("state.empty.task")}</Text>
      {state.description && <Text style={styles.desc}>{state.description}</Text>}
    </View>
  );
}
