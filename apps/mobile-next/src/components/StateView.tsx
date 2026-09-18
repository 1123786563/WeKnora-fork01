import React from "react";
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "./Icon";

// 通用页面状态（02 规格固定文案）。loading 骨架无假业务计数；error 保留重试；
// forbidden 不展示敏感正文；empty 只描述当前 scope。
export type ViewState =
  | { kind: "loading" }
  | { kind: "empty"; title?: string; description?: string }
  | { kind: "error"; retry?: () => void }
  | { kind: "forbidden" }
  | { kind: "unknown" };

export function StateView({ state, testID }: { state: ViewState; testID?: string }) {
  const { theme } = useTheme();
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
      <View style={styles.wrap} testID={testID ?? "state-loading"} accessibilityLabel="正在同步当前空间">
        <ActivityIndicator size="small" color={theme.c.brand} />
        <Text style={styles.desc}>正在同步当前空间</Text>
      </View>
    );
  }
  if (state.kind === "forbidden") {
    return (
      <View style={styles.wrap} testID={testID ?? "state-forbidden"}>
        <Icon name="lock" size={24} color={theme.c.subtle} />
        <Text style={styles.title}>无法访问这项资源</Text>
      </View>
    );
  }
  if (state.kind === "unknown") {
    return (
      <View style={styles.wrap} testID={testID ?? "state-unknown"}>
        <Icon name="refresh" size={24} color={theme.c.subtle} />
        <Text style={styles.title}>等待同步</Text>
        <Text style={styles.desc}>正在核实原请求，请勿重复提交</Text>
      </View>
    );
  }
  if (state.kind === "error") {
    return (
      <View style={styles.wrap} testID={testID ?? "state-error"}>
        <Icon name="alert" size={24} color={theme.c.danger} />
        <Text style={styles.title}>暂时无法连接，稍后重试</Text>
        {state.retry && (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="重试"
            style={styles.retry}
            onPress={state.retry}
          >
            <Text style={styles.retryText}>重试</Text>
          </Pressable>
        )}
      </View>
    );
  }
  return (
    <View style={styles.wrap} testID={testID ?? "state-empty"}>
      <Icon name="spark" size={24} color={theme.c.subtle} />
      <Text style={styles.title}>{state.title ?? "这里还没有任务"}</Text>
      {state.description && <Text style={styles.desc}>{state.description}</Text>}
    </View>
  );
}
