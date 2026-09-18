import React, { useCallback, useEffect, useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { ActionButton } from "@/components/ActionButton";
import { DecisionSheet } from "@/components/DecisionSheet";
import { RunStatusStack, type RunStatusItem } from "@/components/RunStatusStack";
import { StateView } from "@/components/StateView";
import { StatusBadge } from "@/components/StatusBadge";
import type { RunViewWire } from "@/contracts/workbench";
import { formatClock, runStatusLabel, runStatusTone } from "@/features/workbench/labels";

// M08 执行详情（02 规格）：GET /workbench/executions/{id}(+snapshot)；
// 三状态行（run / execution / settlement）+ 预算卡 + 简化时间线 + 取消（需 revision，409=已在另一端处理）。
// 说明（D-06）：execution/settlement 观察值后端尚未提供——execution 显示"等待同步"、settlement 显示"待对账"。

type PageView = "loading" | "ready" | "forbidden" | "notfound" | "error";

export default function ExecutionScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{ runId: string }>();
  const runId = typeof params.runId === "string" ? params.runId : "";

  const [run, setRun] = useState<RunViewWire | null>(null);
  const [title, setTitle] = useState("任务详情");
  const [view, setView] = useState<PageView>("loading");
  const [cancelSheet, setCancelSheet] = useState(false);
  const [cancelBusy, setCancelBusy] = useState(false);
  const [conflictNote, setConflictNote] = useState<string | null>(null);

  const load = useCallback(async () => {
    // 视觉验证 fixture（显式开关，非生产默认）：演示一条进行中任务
    if (app.visualFixture) {
      setRun({
        run_id: runId || "run_demo_018",
        session_id: "",
        status: "running",
        revision: 3,
        budget_upper: 100,
        spent: 38,
        as_of: new Date().toISOString(),
      });
      setTitle("整理产品反馈");
      setView("ready");
      return;
    }
    setView("loading");
    setConflictNote(null);
    try {
      const base = await app.api.getExecution(runId);
      let current = base;
      try {
        // snapshot 尽力而为：失败时回退 getExecution 结果（不冒充同步成功）
        const snap = await app.api.snapshot(runId);
        if (snap.run?.run_id) current = snap.run;
      } catch {
        // snapshot 不可用不阻塞详情
      }
      setRun(current);
      setView("ready");
      // 会话标题尽力而为：仅用于展示，读取失败不阻塞
      if (current.session_id) {
        try {
          const sessions = await app.api.sessions({ page: 1, page_size: 50 });
          const s = sessions.sessions.find((x) => x.id === current.session_id);
          if (s?.title) setTitle(s.title);
        } catch {
          // 忽略
        }
      }
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "forbidden") setView("forbidden");
      else if (err.kind === "not_found") setView("notfound");
      else setView("error");
    }
  }, [app, runId]);

  useEffect(() => {
    void load();
  }, [load]);

  const doCancel = async () => {
    if (!run || run.revision == null) return;
    setCancelBusy(true);
    try {
      const updated = await app.api.command(runId, "cancel", run.revision);
      setRun(updated);
      setCancelSheet(false);
      setConflictNote(null);
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "conflict" || err.kind === "cursor_expired") {
        setConflictNote("已在另一端处理");
        setCancelSheet(false);
      } else {
        setConflictNote("暂时无法连接，稍后重试");
      }
    } finally {
      setCancelBusy(false);
    }
  };

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[4],
      paddingTop: insets.top + theme.space[12],
      paddingHorizontal: theme.space[8],
      paddingBottom: theme.space[8],
    },
    headerTitle: {
      flex: 1,
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32], gap: theme.space[16] },
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      gap: theme.space[8],
    },
    titleRow: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      gap: theme.space[8],
    },
    cardTitle: {
      flex: 1,
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    runId: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
    },
    soft: { backgroundColor: theme.c["surface-alt"] },
    detailRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      minHeight: 32,
    },
    detailLabel: {
      width: 92,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
    },
    detailValue: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      fontWeight: "600",
      color: theme.c.ink,
    },
    sectionTitle: {
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    timelineItem: {
      flexDirection: "row",
      gap: theme.space[12],
    },
    timelineRail: { alignItems: "center", width: 12 },
    timelineDot: {
      width: 10,
      height: 10,
      borderRadius: theme.radius.pill,
      backgroundColor: theme.c.brand,
      marginTop: 4,
    },
    timelineDotPending: {
      backgroundColor: "transparent",
      borderWidth: 2,
      borderColor: theme.c["control-line"],
    },
    timelineLine: { flex: 1, width: 2, backgroundColor: theme.c.line, marginVertical: 2 },
    timelineBody: { flex: 1, gap: 2, paddingBottom: theme.space[16] },
    timelineTime: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
    },
    timelineTitle: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      fontWeight: "600",
      color: theme.c.ink,
    },
    timelineDesc: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
    },
    notice: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["warning-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
    },
    noticeText: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.warning,
    },
    hint: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
      textAlign: "center",
    },
  });

  if (view === "loading") {
    return (
      <View style={styles.wrap}>
        <View style={styles.header}>
          <IconButton name="back" label="返回" onPress={() => router.back()} />
          <Text style={styles.headerTitle} accessibilityRole="header">
            执行详情
          </Text>
        </View>
        <StateView state={{ kind: "loading" }} />
      </View>
    );
  }
  if (view === "forbidden" || view === "notfound") {
    return (
      <View style={styles.wrap}>
        <View style={styles.header}>
          <IconButton name="back" label="返回" onPress={() => router.back()} />
          <Text style={styles.headerTitle} accessibilityRole="header">
            执行详情
          </Text>
        </View>
        <StateView
          state={
            view === "forbidden"
              ? { kind: "forbidden" }
              : { kind: "empty", title: "没有找到这个任务", description: "执行可能已被清理，或链接来自其他空间。" }
          }
        />
      </View>
    );
  }
  if (view === "error" || !run) {
    return (
      <View style={styles.wrap}>
        <View style={styles.header}>
          <IconButton name="back" label="返回" onPress={() => router.back()} />
          <Text style={styles.headerTitle} accessibilityRole="header">
            执行详情
          </Text>
        </View>
        <StateView state={{ kind: "error", retry: () => void load() }} />
      </View>
    );
  }

  const statusItems: RunStatusItem[] = [
    {
      key: "run",
      title: "产品任务",
      rawValue: run.status,
      label: runStatusLabel(run.status),
      tone: runStatusTone(run.status),
      unknown: run.status === "unknown",
    },
    {
      key: "execution",
      title: "执行观察",
      rawValue: "unknown",
      label: "等待同步",
      tone: "unknown",
      unknown: true,
    },
    {
      key: "settlement",
      title: "费用结算",
      rawValue: "pending",
      label: "待对账 · 非最终消耗",
      tone: "attention",
      unknown: false,
    },
  ];

  const lastSync = formatClock(run.as_of);
  const timeline = [
    {
      time: null as string | null,
      titleText: "任务请求已受理",
      desc: "已绑定唯一请求，不重复启动。",
      pending: false,
    },
    {
      time: null as string | null,
      titleText: "知识检索已完成",
      desc: "检索当前空间中获准使用的资料。",
      pending: false,
    },
    {
      time: lastSync,
      titleText: runStatusLabel(run.status),
      desc: "产品状态、执行观察与结算分别更新。",
      pending: true,
    },
  ];

  const cancelDisabled = run.revision == null;

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle} numberOfLines={1} accessibilityRole="header">
          执行详情
        </Text>
      </View>
      <ScrollView contentContainerStyle={styles.scroll}>
        <View style={styles.card}>
          <View style={styles.titleRow}>
            <Text style={styles.cardTitle} numberOfLines={2}>
              {title}
            </Text>
            <StatusBadge
              domain="run"
              label={run.status === "unknown" ? "待核实" : "平台执行"}
              tone="neutral"
            />
          </View>
          <Text style={styles.runId} numberOfLines={1} ellipsizeMode="middle">
            {run.run_id ? `执行 ID ${run.run_id}` : "执行 ID 尚未确认"}
          </Text>
        </View>

        <RunStatusStack items={statusItems} lastSyncAt={lastSync} />

        {conflictNote && (
          <View style={styles.notice} accessibilityRole="alert">
            <Icon name="alert" size={16} color={theme.c.warning} />
            <Text style={styles.noticeText}>{conflictNote}</Text>
          </View>
        )}

        <View style={[styles.card, styles.soft]}>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>预算上限</Text>
            <Text style={styles.detailValue}>
              {run.budget_upper != null ? `${run.budget_upper} Credits` : "等待同步"}
            </Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>已记录消耗</Text>
            <Text style={styles.detailValue}>
              {run.spent != null ? `${run.spent} Credits` : "等待同步"}
            </Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>最后同步</Text>
            <Text style={styles.detailValue}>{lastSync ?? "尚未同步"}</Text>
          </View>
        </View>

        <Text style={styles.sectionTitle} accessibilityRole="header">
          执行时间线
        </Text>
        <View style={styles.card}>
          {timeline.map((it, idx) => (
            <View key={it.titleText + idx} style={styles.timelineItem}>
              <View style={styles.timelineRail}>
                <View style={[styles.timelineDot, it.pending && styles.timelineDotPending]} />
                {idx < timeline.length - 1 && <View style={styles.timelineLine} />}
              </View>
              <View style={styles.timelineBody}>
                <Text style={styles.timelineTime}>{it.time ?? "—"}</Text>
                <Text style={styles.timelineTitle}>{it.titleText}</Text>
                <Text style={styles.timelineDesc}>{it.desc}</Text>
              </View>
            </View>
          ))}
        </View>

        <ActionButton
          label="申请取消任务"
          variant="secondary"
          block
          disabled={cancelDisabled}
          onPress={() => setCancelSheet(true)}
          accessibilityHint="取消已受理不等于停止已确认，也不等于退款"
          testID="cancel-run-button"
        />
        {cancelDisabled && <Text style={styles.hint}>缺少版本信息，暂不能取消；下拉或稍后刷新重试。</Text>}
      </ScrollView>

      <DecisionSheet
        visible={cancelSheet}
        title="申请取消任务"
        impact="取消已受理不等于停止已确认，也不等于退款。"
        frozenSummary={[
          `任务：${title}`,
          `执行 ID：${run.run_id || "尚未确认"}`,
          `当前状态：${runStatusLabel(run.status)}`,
          `revision ${run.revision ?? ""}`,
        ]}
        confirm={{ label: "确认申请取消", variant: "danger", busy: cancelBusy, onPress: () => void doCancel() }}
        dismiss={{ label: "再想想", variant: "secondary", onPress: () => setCancelSheet(false) }}
        testID="cancel-decision-sheet"
      />
    </View>
  );
}
