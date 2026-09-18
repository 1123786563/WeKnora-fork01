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
import { StateView } from "@/components/StateView";
import { StatusBadge } from "@/components/StatusBadge";
import type { InteractionWire } from "@/contracts/workbench";
import { kindLabel } from "@/features/workbench/labels";

// M09 审批（02 规格）：GET interactions(runId) 按 id 定位；批准/拒绝走 decide。
// 409（冲突）→ "内容已在另一端改变，请刷新"，按钮禁用、Sheet 标记 stale。

type PageView = "loading" | "ready" | "forbidden" | "notfound" | "error";

function payloadText(payload: Record<string, unknown> | null, keys: string[], fallback: string): string {
  if (!payload) return fallback;
  for (const k of keys) {
    const v = payload[k];
    if (typeof v === "string" && v.length > 0) return v;
  }
  return fallback;
}

export default function ApprovalScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{ interactionId: string; runId: string }>();
  const interactionId = typeof params.interactionId === "string" ? params.interactionId : "";
  const runId = typeof params.runId === "string" ? params.runId : "";

  const [interaction, setInteraction] = useState<InteractionWire | null>(null);
  const [view, setView] = useState<PageView>("loading");
  const [sheetMode, setSheetMode] = useState<null | "approve" | "reject">(null);
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [doneNote, setDoneNote] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!interactionId || !runId) {
      setView("notfound");
      return;
    }
    setView("loading");
    setConflict(false);
    try {
      const list = await app.api.interactions(runId);
      const found = list.find((i) => i.id === interactionId) ?? null;
      if (!found) {
        setView("notfound");
        return;
      }
      setInteraction(found);
      setView("ready");
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "forbidden") setView("forbidden");
      else setView("error");
    }
  }, [app, interactionId, runId]);

  useEffect(() => {
    void load();
  }, [load]);

  const doDecide = async () => {
    if (!interaction || !sheetMode) return;
    setBusy(true);
    try {
      const updated = await app.api.decide(interaction.id, {
        pending_id: interaction.id,
        expected_revision: interaction.revision ?? 0,
        action: sheetMode === "approve" ? "approve" : "reject",
      });
      setInteraction(updated);
      setDoneNote(
        sheetMode === "approve"
          ? "决定已记录，后续执行结果仍等待服务端确认。"
          : "已拒绝这次操作。任务可能进入待处理状态。",
      );
      setSheetMode(null);
      setConflict(false);
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "conflict" || err.kind === "cursor_expired") {
        setConflict(true);
        setDoneNote(null);
        // 保持 Sheet 打开并以 stale 标记禁用确认，由用户刷新
      } else {
        setDoneNote("暂时无法连接，稍后重试");
      }
    } finally {
      setBusy(false);
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
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[24], gap: theme.space[16] },
    warningNotice: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["warning-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
    },
    warningNoticeText: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.warning,
    },
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
    detailRow: {
      flexDirection: "row",
      alignItems: "flex-start",
      gap: theme.space[8],
      minHeight: 30,
    },
    detailLabel: {
      width: 76,
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
    quote: {
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["surface-alt"],
      padding: theme.space[16],
    },
    quoteText: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.ink,
    },
    soft: { backgroundColor: theme.c["surface-alt"] },
    softTitle: {
      fontSize: theme.type.label.fontSize,
      lineHeight: theme.type.label.lineHeight,
      fontWeight: theme.type.label.fontWeight as "600",
      color: theme.c.ink,
    },
    softText: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
      marginTop: theme.space[4],
    },
    footer: {
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      backgroundColor: theme.c.bg,
      paddingHorizontal: theme.space[20],
      paddingTop: theme.space[12],
      paddingBottom: insets.bottom + theme.space[12],
      flexDirection: "row",
      gap: theme.space[12],
    },
    footerCell: { flex: 1 },
    doneNote: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
    },
    doneNoteText: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"],
    },
  });

  const header = (
    <View style={styles.header}>
      <IconButton name="back" label="返回" onPress={() => router.back()} />
      <Text style={styles.headerTitle} accessibilityRole="header">
        审批
      </Text>
    </View>
  );

  if (view === "loading") {
    return (
      <View style={styles.wrap}>
        {header}
        <StateView state={{ kind: "loading" }} />
      </View>
    );
  }
  if (view === "forbidden") {
    return (
      <View style={styles.wrap}>
        {header}
        <StateView state={{ kind: "forbidden" }} />
      </View>
    );
  }
  if (view === "notfound") {
    return (
      <View style={styles.wrap}>
        {header}
        <StateView
          state={{
            kind: "empty",
            title: "没有找到这条待处理",
            description: "它可能已被处理、过期，或来自其他空间。",
          }}
        />
      </View>
    );
  }
  if (view === "error" || !interaction) {
    return (
      <View style={styles.wrap}>
        {header}
        <StateView state={{ kind: "error", retry: () => void load() }} />
      </View>
    );
  }

  const payload = interaction.payload;
  const target = payloadText(payload, ["target", "target_label", "destination", "scope", "channel"], "需在线刷新确认");
  const summary = interaction.summary || payloadText(payload, ["summary", "content", "text"], "");
  const connection = payloadText(payload, ["connection", "connector", "app", "provider"], "需在线刷新确认");
  const account = payloadText(payload, ["account", "account_label", "executing_account", "actor"], "需在线刷新确认");
  const risk = payloadText(payload, ["risk", "risk_level"], "外部写入，需要明确确认");
  const expiresAt = payloadText(payload, ["expires_at", "valid_until", "expires"], "需在线刷新确认");

  const resolved = interaction.status !== "pending";
  const actionsDisabled = resolved || conflict || busy;

  return (
    <View style={styles.wrap}>
      {header}
      <ScrollView contentContainerStyle={styles.scroll}>
        <View style={styles.warningNotice} accessibilityRole="alert">
          <Icon name="shield" size={16} color={theme.c.warning} />
          <Text style={styles.warningNoticeText}>请核对完整操作。批准仅对本次目标与内容生效。</Text>
        </View>

        <View style={styles.card}>
          <View style={styles.titleRow}>
            <Text style={styles.cardTitle} numberOfLines={2}>
              {interaction.title || "一次 Agent 操作需要确认"}
            </Text>
            <StatusBadge
              domain="interaction"
              label={conflict ? "版本已变化" : resolved ? "已处理" : "待决定"}
              tone={conflict ? "danger" : resolved ? "success" : "attention"}
            />
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>类型</Text>
            <Text style={styles.detailValue}>{kindLabel(interaction.kind)}</Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>连接</Text>
            <Text style={styles.detailValue}>{connection}</Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>执行账号</Text>
            <Text style={styles.detailValue}>{account}</Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>发布目标</Text>
            <Text style={styles.detailValue}>{target}</Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>操作风险</Text>
            <Text style={styles.detailValue}>{risk}</Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>内容版本</Text>
            <Text style={styles.detailValue}>
              {interaction.revision != null ? `revision ${interaction.revision}` : "需在线刷新确认"}
            </Text>
          </View>
          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>有效期</Text>
            <Text style={styles.detailValue}>{expiresAt}</Text>
          </View>
        </View>

        {conflict && (
          <View style={styles.warningNotice} accessibilityRole="alert">
            <Icon name="alert" size={16} color={theme.c.warning} />
            <Text style={styles.warningNoticeText}>内容已在另一端改变，请刷新。</Text>
          </View>
        )}

        {summary.length > 0 && (
          <View>
            <Text style={styles.sectionTitle} accessibilityRole="header">
              将要发布的内容
            </Text>
            <View style={styles.quote}>
              <Text style={styles.quoteText}>{summary}</Text>
            </View>
          </View>
        )}

        <View style={[styles.card, styles.soft]}>
          <Text style={styles.softTitle}>这次批准的边界</Text>
          <Text style={styles.softText}>只批准本页内容，不授予长期自动发布权限，也不会提高任务预算。</Text>
        </View>

        {doneNote && (
          <View style={styles.doneNote} accessibilityRole="alert">
            <Icon name="checkcircle" size={16} color={theme.mode === "light" ? theme.c.brand : theme.c.accent} />
            <Text style={styles.doneNoteText}>{doneNote}</Text>
          </View>
        )}
      </ScrollView>

      <View style={styles.footer}>
        <View style={styles.footerCell}>
          <ActionButton
            label="拒绝"
            variant="secondary"
            block
            disabled={actionsDisabled}
            onPress={() => setSheetMode("reject")}
            accessibilityHint="拒绝后本次操作不会执行"
            testID="reject-button"
          />
        </View>
        <View style={styles.footerCell}>
          <ActionButton
            label="批准这一次"
            variant="primary"
            block
            disabled={actionsDisabled}
            onPress={() => setSheetMode("approve")}
            accessibilityHint="仅对本次目标与内容生效"
            testID="approve-button"
          />
        </View>
      </View>

      <DecisionSheet
        visible={sheetMode !== null}
        title={sheetMode === "approve" ? "批准这一次" : "拒绝这次操作"}
        impact={
          sheetMode === "approve"
            ? "批准仅对本次目标与内容生效，不授予长期授权。"
            : "拒绝后本次操作不会执行；任务可能进入待处理状态。"
        }
        frozenSummary={[
          `目标：${target}`,
          summary ? `内容摘要：${summary}` : "内容摘要：需在线刷新确认",
          `revision ${interaction.revision ?? "待确认"}`,
        ]}
        stale={conflict}
        confirm={{
          label: sheetMode === "approve" ? "确认批准" : "确认拒绝",
          variant: sheetMode === "approve" ? "primary" : "danger",
          busy,
          onPress: () => void doDecide(),
        }}
        dismiss={{ label: "返回核对", variant: "secondary", onPress: () => setSheetMode(null) }}
        testID="approval-decision-sheet"
      />
    </View>
  );
}
