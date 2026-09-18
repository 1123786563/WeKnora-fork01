import React, { useCallback, useEffect, useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { StatusBadge } from "@/components/StatusBadge";
import { StateView } from "@/components/StateView";
import { SectionHeader } from "@/components/SearchField";
import type { UsageSummaryWire } from "@/contracts/workbench";

// M18 空间用量：可用额度/预占/待结算/已结算分开显示；待结算不是最终消耗；不提供支付/充值。
const FIXTURE_USAGE: UsageSummaryWire = {
  as_of: "今天 10:15",
  period: "本月",
  available: 100,
  held: 22,
  pending: 0,
  settled: 38,
  unit: "Credits",
};

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; usage: UsageSummaryWire }
  | { kind: "error"; retry: () => void }
  | { kind: "forbidden" };

export default function UsageScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  const load = useCallback(async () => {
    if (app.visualFixture) {
      setState({ kind: "ready", usage: FIXTURE_USAGE });
      return;
    }
    setState({ kind: "loading" });
    try {
      const usage = await app.api.usageSummary();
      setState({ kind: "ready", usage });
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
    periodRow: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      marginBottom: theme.space[16],
    },
    periodText: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    metrics: {
      flexDirection: "row",
      flexWrap: "wrap",
      gap: theme.space[12],
      marginBottom: theme.space[8],
    },
    metricCard: {
      width: "48%" as const,
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      alignItems: "flex-start",
      gap: theme.space[4],
    },
    metricValue: { fontSize: theme.type.metric.fontSize, lineHeight: theme.type.metric.lineHeight, fontWeight: "700", color: theme.c.ink },
    metricLabel: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    asOfRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      marginTop: theme.space[8],
    },
    asOfText: { fontSize: theme.type.caption.fontSize, color: theme.c.subtle },
    section: { marginTop: theme.space[24] },
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
    forbiddenText: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
      textAlign: "center",
      paddingHorizontal: theme.space[24],
    },
  });

  const spaceName =
    app.identity?.memberships.find((m) => m.tenantId === app.identity?.selectedTenantId)?.tenantName ?? "当前空间";

  const formatValue = (v: number | null): string => {
    if (v == null) return "—";
    return Number.isInteger(v) ? v.toLocaleString() : v.toFixed(1);
  };

  const metricCards = (usage: UsageSummaryWire): Array<{ label: string; value: number | null }> => [
    { label: "可用额度", value: usage.available },
    { label: "预占", value: usage.held },
    { label: "待结算", value: usage.pending },
    { label: "已结算", value: usage.settled },
  ];

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle}>空间用量</Text>
      </View>

      {state.kind === "loading" && <StateView state={{ kind: "loading" }} />}
      {state.kind === "error" && <StateView state={{ kind: "error", retry: state.retry }} />}
      {state.kind === "forbidden" && (
        <>
          <StateView state={{ kind: "forbidden" }} />
          <Text style={styles.forbiddenText}>账单权限不足，无法查看明细</Text>
        </>
      )}

      {state.kind === "ready" && (
        <ScrollView contentContainerStyle={styles.scroll}>
          <View style={styles.periodRow}>
            <Text style={styles.periodText}>{spaceName}</Text>
            <StatusBadge domain="settlement" label={state.usage.period ?? "本月"} tone="neutral" />
          </View>

          <View style={styles.metrics}>
            {metricCards(state.usage).map((m) => (
              <View key={m.label} style={styles.metricCard} accessibilityLabel={`${m.label} ${m.value == null ? "未知" : m.value}`}>
                <Text style={styles.metricValue}>{formatValue(m.value)}</Text>
                <Text style={styles.metricLabel}>{m.label}</Text>
              </View>
            ))}
          </View>
          <Text style={styles.asOfText}>
            计价单位 {state.usage.unit} · 数据截至 {state.usage.as_of ?? "待同步"}
          </Text>

          <View style={styles.section}>
            <SectionHeader title="口径说明" />
            <View style={styles.notice}>
              <Icon name="shield" size={16} color={theme.c.brand} />
              <Text style={styles.noticeText}>预占与已结算分开显示；待结算不是最终消耗。</Text>
            </View>
            <View style={styles.notice}>
              <Icon name="info" size={16} color={theme.c.brand} />
              <Text style={styles.noticeText}>余额、预占和最终消耗是不同概念；购买与充值不在此页提供。</Text>
            </View>
          </View>
        </ScrollView>
      )}
    </View>
  );
}
