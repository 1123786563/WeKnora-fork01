import React, { useEffect, useRef } from "react";
import { BackHandler, Modal, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon } from "./Icon";
import { ActionButton, type ButtonVariant } from "./ActionButton";

// 确认 Sheet（03 设计系统 §7 + 02 规格 §3.1）：
// - 标题、具体影响、冻结对象摘要、两侧按钮；危险动作不默认回车确认。
// - Android 返回键关闭且不提交；背景不可交互；内容超长内部滚动、操作始终可达。
// - frozenDigest/expectedRevision 变化时上层应关闭（本组件以 stale 标记禁用提交）。
export interface DecisionAction {
  label: string;
  variant: ButtonVariant;
  onPress: () => void;
  busy?: boolean;
}

export interface DecisionSheetProps {
  visible: boolean;
  title: string;
  impact: string; // 具体影响描述
  frozenSummary: string[]; // 冻结对象摘要行（scope/目标/内容摘要/版本/时效）
  confirm: DecisionAction;
  dismiss: DecisionAction;
  stale?: boolean; // revision/digest 变化 → 禁用确认并提示刷新
  testID?: string;
}

export function DecisionSheet({
  visible,
  title,
  impact,
  frozenSummary,
  confirm,
  dismiss,
  stale = false,
  testID,
}: DecisionSheetProps) {
  const { theme } = useTheme();

  // Android 物理返回：关闭且不提交（02 规格 §1）
  useEffect(() => {
    if (!visible) return;
    const sub = BackHandler.addEventListener("hardwareBackPress", () => {
      dismiss.onPress();
      return true;
    });
    return () => sub.remove();
  }, [visible, dismiss]);

  const styles = StyleSheet.create({
    scrim: { flex: 1, backgroundColor: theme.c.scrim, justifyContent: "flex-end" },
    sheet: {
      backgroundColor: theme.c.surface,
      borderTopLeftRadius: theme.radius.sheet,
      borderTopRightRadius: theme.radius.sheet,
      paddingHorizontal: theme.space[20],
      paddingBottom: theme.space[24],
      paddingTop: theme.space[8],
      maxHeight: "85%" as const,
    },
    grabber: {
      alignSelf: "center" as const,
      width: 40,
      height: 4,
      borderRadius: theme.radius.pill,
      backgroundColor: theme.c.line,
      marginBottom: theme.space[12],
    },
    title: {
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    impact: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
      marginTop: theme.space[4],
    },
    frozenWrap: {
      marginTop: theme.space[12],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["surface-alt"],
      padding: theme.space[12],
      gap: theme.space[4],
    },
    frozenRow: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.ink,
    },
    staleBanner: {
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[6],
      marginTop: theme.space[12],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["warning-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    staleText: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.warning,
    },
    actions: {
      flexDirection: "row" as const,
      gap: theme.space[12],
      marginTop: theme.space[16],
    },
  });

  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={dismiss.onPress} testID={testID ?? "decision-sheet"}>
      <Pressable style={styles.scrim} onPress={dismiss.onPress} accessibilityLabel="关闭确认层">
        {/* 内层 Pressable 承接按压，避免点内容关闭 */}
        <Pressable style={styles.sheet}>
          <View style={styles.grabber} />
          <Text style={styles.title} accessibilityRole="header">
            {title}
          </Text>
          <Text style={styles.impact}>{impact}</Text>
          <ScrollView style={{ marginTop: theme.space[4] }} nestedScrollEnabled>
            <View style={styles.frozenWrap}>
              {frozenSummary.map((line) => (
                <Text key={line} style={styles.frozenRow}>
                  {line}
                </Text>
              ))}
            </View>
            {stale && (
              <View style={styles.staleBanner} accessibilityRole="alert">
                <Icon name="alert" size={16} color={theme.c.warning} />
                <Text style={styles.staleText}>内容已在另一端改变，请刷新后再决定</Text>
              </View>
            )}
          </ScrollView>
          <View style={styles.actions}>
            <View style={{ flex: 1 }}>
              <ActionButton {...dismiss} block />
            </View>
            <View style={{ flex: 1 }}>
              <ActionButton {...confirm} block disabled={stale || confirm.busy} />
            </View>
          </View>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
