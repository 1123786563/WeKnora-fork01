import React, { useCallback, useEffect, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { StatusBadge } from "@/components/StatusBadge";
import { StateView } from "@/components/StateView";
import { SectionHeader } from "@/components/SearchField";
import { targetSelection } from "@/features/targets/selection";
import type { BadgeSpec } from "@/components/StatusBadge";
import type { ExecutionTargetWire } from "@/contracts/workbench";

// M15 执行目标：任务在哪里运行由服务端受权引用决定；手机端不提供任意节点地址输入。
const FIXTURE_TARGETS: ExecutionTargetWire[] = [
  { id: "platform-default", name: "平台默认", status: "available", capabilities: ["可观察", "可申请停止"], unavailable_reason: null },
  { id: "managed-sandbox", name: "受控托管节点", status: "available", capabilities: ["隔离工作目录", "可观察"], unavailable_reason: null },
];

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; targets: ExecutionTargetWire[] }
  | { kind: "empty" }
  | { kind: "error"; retry: () => void }
  | { kind: "forbidden" };

export default function TargetsScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [selectedId, setSelectedId] = useState<string | null>(targetSelection.current?.id ?? null);

  const load = useCallback(async () => {
    if (app.visualFixture) {
      setState({ kind: "ready", targets: FIXTURE_TARGETS });
      return;
    }
    setState({ kind: "loading" });
    try {
      const targets = await app.api.executionTargets();
      if (!targets.length) {
        setState({ kind: "empty" });
        return;
      }
      setState({ kind: "ready", targets });
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "forbidden") {
        setState({ kind: "forbidden" });
        return;
      }
      setState({ kind: "error", retry: () => void load() });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app.visualFixture]);

  useEffect(() => {
    void load();
  }, [load]);

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      paddingTop: insets.top + theme.space[12],
      paddingBottom: theme.space[12],
      paddingHorizontal: theme.space[20],
    },
    headerTitle: { fontSize: theme.type.subtitle.fontSize, lineHeight: theme.type.subtitle.lineHeight, fontWeight: "600", color: theme.c.ink },
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      borderWidth: 2,
      borderColor: "transparent",
      padding: theme.space[16],
      gap: theme.space[12],
      marginBottom: theme.space[12],
    },
    cardSelected: { borderColor: theme.c.brand },
    cardDisabled: { opacity: 0.7 },
    row: { flexDirection: "row", alignItems: "center", gap: theme.space[12] },
    iconWrap: {
      width: 44,
      height: 44,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    iconWrapNeutral: { backgroundColor: theme.c["surface-alt"] },
    name: { fontSize: theme.type["body-sm"].fontSize + 1, lineHeight: theme.type["body-sm"].lineHeight, fontWeight: "600", color: theme.c.ink },
    checkRing: {
      width: 26,
      height: 26,
      borderRadius: theme.radius.pill,
      borderWidth: 2,
      borderColor: theme.c.brand,
      alignItems: "center",
      justifyContent: "center",
    },
    chips: { flexDirection: "row", flexWrap: "wrap", gap: theme.space[8] },
    chip: {
      minHeight: 32,
      borderRadius: theme.radius.pill,
      paddingHorizontal: theme.space[12],
      alignItems: "center",
      justifyContent: "center",
      backgroundColor: theme.c["surface-alt"],
    },
    chipText: { fontSize: theme.type.caption.fontSize, lineHeight: theme.type.caption.lineHeight, color: theme.c.muted },
    reasonRow: { flexDirection: "row", alignItems: "flex-start", gap: theme.space[8] },
    reasonLabel: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.danger, fontWeight: "600" },
    reasonText: { flex: 1, fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight, color: theme.c.muted },
    notice: {
      marginTop: theme.space[20],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
      flexDirection: "row",
      gap: theme.space[8],
      alignItems: "flex-start",
    },
    noticeText: { flex: 1, color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    hint: { fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight, color: theme.c.muted, marginTop: theme.space[8] },
  });

  const statusBadge = (status: string): BadgeSpec => {
    if (status === "available" || status === "active" || status === "ready") return { domain: "run", label: "可用", tone: "success" };
    if (status === "unavailable" || status === "disabled") return { domain: "run", label: "不可用", tone: "neutral" };
    return { domain: "run", label: "等待同步", tone: "unknown", unknown: true };
  };

  const isSelectable = (t: ExecutionTargetWire) =>
    !t.unavailable_reason && (t.status === "available" || t.status === "active" || t.status === "ready");

  const pick = (t: ExecutionTargetWire) => {
    targetSelection.current = { id: t.id, name: t.name };
    setSelectedId(t.id);
    router.back();
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle}>执行目标</Text>
      </View>

      {state.kind === "loading" && <StateView state={{ kind: "loading" }} />}
      {state.kind === "forbidden" && <StateView state={{ kind: "forbidden" }} />}
      {state.kind === "error" && <StateView state={{ kind: "error", retry: state.retry }} />}
      {state.kind === "empty" && (
        <StateView state={{ kind: "empty", title: "暂无可用的执行目标", description: "请联系空间管理员确认平台执行或托管节点授权。" }} />
      )}

      {state.kind === "ready" && (
        <ScrollView contentContainerStyle={styles.scroll}>
          <SectionHeader title="选择任务运行的位置" />
          {state.targets.map((t) => {
            const selectable = isSelectable(t);
            const selected = t.id === selectedId;
            return (
              <Pressable
                key={t.id}
                accessibilityRole="button"
                accessibilityLabel={`执行目标 ${t.name}`}
                accessibilityState={{ selected, disabled: !selectable }}
                disabled={!selectable}
                onPress={() => pick(t)}
                style={[styles.card, selected && styles.cardSelected, !selectable && styles.cardDisabled]}
              >
                <View style={styles.row}>
                  <View style={[styles.iconWrap, !selectable && styles.iconWrapNeutral]}>
                    <Icon name="monitor" size={22} color={selectable ? theme.c.brand : theme.c.muted} />
                  </View>
                  <View style={{ flex: 1 }}>
                    <Text style={styles.name}>{t.name}</Text>
                    <View style={{ marginTop: theme.space[4] }}>
                      <StatusBadge {...statusBadge(t.status)} />
                    </View>
                  </View>
                  {selected && (
                    <View style={styles.checkRing}>
                      <Icon name="check" size={14} color={theme.c.brand} />
                    </View>
                  )}
                </View>
                {t.capabilities.length > 0 && (
                  <View style={styles.chips}>
                    {t.capabilities.map((cap) => (
                      <View key={cap} style={styles.chip}>
                        <Text style={styles.chipText}>{cap}</Text>
                      </View>
                    ))}
                  </View>
                )}
                {t.unavailable_reason && (
                  <View style={styles.reasonRow}>
                    <Text style={styles.reasonLabel}>不可用原因</Text>
                    <Text style={styles.reasonText}>{t.unavailable_reason}</Text>
                  </View>
                )}
              </Pressable>
            );
          })}

          <View style={styles.notice}>
            <Icon name="shield" size={16} color={theme.c.brand} />
            <Text style={styles.noticeText}>手机只选择服务端受权引用，不提供任意节点地址输入。</Text>
          </View>
          <Text style={styles.hint}>个人节点、实时终端与私密 E2EE 是独立能力；不能由选择目标自动启用。</Text>
        </ScrollView>
      )}
    </View>
  );
}
