import React, { useEffect, useState } from "react";
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router, useFocusEffect } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { ActionButton } from "@/components/ActionButton";
import { FormField } from "@/components/FormField";
import { OfflineNotice } from "@/components/OfflineNotice";
import { DecisionSheet } from "@/components/DecisionSheet";
import type { IconName } from "@/components/Icon";
import type { SubmitOutcome } from "@/features/workbench/submit/SubmissionService";
import { agentSelection } from "@/features/workbench/agentSelection";
import { targetSelection } from "@/features/targets/selection";

// M05 新建任务（02 规格）：Agent、任务描述、附件、知识、执行目标、预算；提交走持久状态机。
const DRAFT_ID = "new-task";

export default function NewTaskScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [agentId, setAgentId] = useState("agent_research");
  const [agentName, setAgentName] = useState("知识研究员");
  const [text, setText] = useState("");
  const [targetId, setTargetId] = useState("platform-default");
  const [targetName, setTargetName] = useState("平台执行");
  const [budget, setBudget] = useState("100");
  const [submitting, setSubmitting] = useState(false);
  const [offline, setOffline] = useState(false);
  const [outcome, setOutcome] = useState<SubmitOutcome | null>(null);
  const [uncertainId, setUncertainId] = useState<string | null>(null);

  // M06/M15 选择页回传：每次聚焦读取（back 返回不重新挂载）
  useFocusEffect(
    React.useCallback(() => {
      if (agentSelection.current) {
        setAgentId(agentSelection.current.id);
        setAgentName(agentSelection.current.name);
      }
      if (targetSelection.current) {
        setTargetId(targetSelection.current.id);
        setTargetName(targetSelection.current.name);
      }
    }, []),
  );

  // 草稿持久化（scope 隔离；离线保存不自动补发）
  useEffect(() => {
    const key = app.scopeKey();
    void app.store.readDraft(key, DRAFT_ID).then((saved) => {
      if (saved) {
        try {
          const d = JSON.parse(saved) as { text: string; agentId: string; agentName: string; targetId: string; targetName: string; budget: string };
          setText(d.text);
          setAgentId(d.agentId);
          setAgentName(d.agentName);
          setTargetId(d.targetId);
          setTargetName(d.targetName);
          setBudget(d.budget);
        } catch {
          // 草稿损坏则忽略
        }
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const saveDraft = async () => {
    await app.store.saveDraft(
      app.scopeKey(),
      DRAFT_ID,
      JSON.stringify({ text, agentId, agentName, targetId, targetName, budget }),
    );
  };

  useEffect(() => {
    const t = setTimeout(() => void saveDraft(), 400);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, agentId, targetId, budget]);

  const submit = async () => {
    setSubmitting(true);
    setOffline(false);
    try {
      const out = await app.submissions.submit({
        text,
        agentId,
        targetId,
        workspaceRef: "ws_default",
        budgetUpper: Number(budget),
        knowledgeBaseIds: [],
        sessionId: null,
      });
      setOutcome(out);
      if (out.kind === "accepted") {
        await app.store.saveDraft(app.scopeKey(), DRAFT_ID, ""); // 清空已提交草稿
        router.replace(`/executions/${out.runId}`);
      } else if (out.kind === "uncertain") {
        setUncertainId(out.requestId);
      } else if (out.kindDetail === "scope_readonly") {
        setOffline(true);
      }
    } finally {
      setSubmitting(false);
    }
  };

  const reconcile = async () => {
    if (!uncertainId) return;
    const out = await app.submissions.reconcile(uncertainId);
    setOutcome(out);
    if (out.kind === "accepted") {
      setUncertainId(null);
      router.replace(`/executions/${out.runId}`);
    }
  };

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    scroll: { paddingHorizontal: theme.space[20], paddingTop: insets.top + theme.space[16], paddingBottom: 140 },
    step: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[6],
      marginBottom: theme.space[20],
    },
    stepDot: {
      width: 22,
      height: 22,
      borderRadius: 999,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    stepDotText: { color: theme.c.brand, fontWeight: "700", fontSize: theme.type.caption.fontSize },
    stepText: { color: theme.c.muted, fontSize: theme.type["body-sm"].fontSize },
    fieldLabel: {
      fontSize: theme.type.label.fontSize,
      fontWeight: "600",
      color: theme.c.ink,
      marginBottom: theme.space[8],
      marginTop: theme.space[16],
    },
    selector: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.control,
      borderWidth: 1,
      borderColor: theme.c["control-line"],
      padding: theme.space[12],
      minHeight: 52,
    },
    selectorIcon: {
      width: 36,
      height: 36,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    selectorName: { fontSize: theme.type["body-sm"].fontSize + 1, fontWeight: "600", color: theme.c.ink },
    selectorSub: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 1 },
    chips: { flexDirection: "row", flexWrap: "wrap", gap: theme.space[8], marginTop: theme.space[12] },
    chip: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[6],
      minHeight: 40,
      borderRadius: theme.radius.pill,
      borderWidth: 1,
      borderColor: theme.c.line,
      backgroundColor: theme.c.surface,
      paddingHorizontal: theme.space[12],
    },
    chipText: { color: theme.c.ink, fontSize: theme.type["body-sm"].fontSize },
    footer: {
      position: "absolute",
      bottom: 0,
      left: 0,
      right: 0,
      padding: theme.space[20],
      paddingBottom: insets.bottom + theme.space[12],
      backgroundColor: theme.c.bg,
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
    },
    error: { color: theme.c.danger, marginTop: theme.space[8], fontSize: theme.type["body-sm"].fontSize },
  });

  const rejectedReason = outcome?.kind === "rejected" ? outcome.reason : null;

  return (
    <KeyboardAvoidingView style={styles.wrap} behavior={Platform.OS === "ios" ? "padding" : undefined}>
      <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">
        <View style={styles.step}>
          <View style={styles.stepDot}>
            <Text style={styles.stepDotText}>1</Text>
          </View>
          <Text style={styles.stepText}>明确目标</Text>
          <Text style={styles.stepText}>—</Text>
          <View style={styles.stepDot}>
            <Text style={styles.stepDotText}>2</Text>
          </View>
          <Text style={styles.stepText}>配置与提交</Text>
        </View>

        <Text style={styles.fieldLabel}>使用哪个助手</Text>
        <Pressable accessibilityRole="button" accessibilityLabel={`选择 Agent，当前 ${agentName}`} onPress={() => router.push("/agents")}>
          <View style={styles.selector}>
            <View style={styles.selectorIcon}>
              <Icon name="spark" size={18} color={theme.c.brand} />
            </View>
            <View style={{ flex: 1 }}>
              <Text style={styles.selectorName}>{agentName}</Text>
              <Text style={styles.selectorSub}>来自当前空间目录 · 提交时再次校验版本</Text>
            </View>
            <Icon name="chevron" size={16} color={theme.c.subtle} />
          </View>
        </Pressable>

        <FormField
          label="想完成什么"
          value={text}
          onChangeText={setText}
          placeholder="例如：整理本周客户反馈，给出三个优先处理的问题。"
          hint="目标越清楚，结果越贴近"
          multiline
        />

        <View style={styles.chips}>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="添加附件"
            style={styles.chip}
            onPress={() =>
              setOutcome({ kind: "rejected", requestId: "", reason: "附件上传将在本地后端联调时启用（RW-027）", kindDetail: "validation" })
            }
          >
            <Icon name="paperclip" size={14} color={theme.c.muted} />
            <Text style={styles.chipText}>添加附件</Text>
          </Pressable>
          <Pressable accessibilityRole="button" accessibilityLabel="引用知识，可选" style={styles.chip} onPress={() => router.push("/(tabs)/resources")}>
            <Icon name="book" size={14} color={theme.c.muted} />
            <Text style={styles.chipText}>引用知识（可选）</Text>
          </Pressable>
        </View>

        <Text style={styles.fieldLabel}>在哪里执行</Text>
        <Pressable accessibilityRole="button" accessibilityLabel={`选择执行目标，当前 ${targetName}`} onPress={() => router.push("/targets")}>
          <View style={styles.selector}>
            <View style={[styles.selectorIcon, { backgroundColor: theme.c["surface-alt"] }]}>
              <Icon name="monitor" size={18} color={theme.c.muted} />
            </View>
            <View style={{ flex: 1 }}>
              <Text style={styles.selectorName}>{targetName}</Text>
              <Text style={styles.selectorSub}>{targetId === "platform-default" ? "无需绑定个人电脑" : "独立沙箱 · 服务端引用"}</Text>
            </View>
            <Icon name="chevron" size={16} color={theme.c.subtle} />
          </View>
        </Pressable>

        <FormField
          label="预算上限"
          value={budget}
          onChangeText={(v) => setBudget(v.replace(/[^0-9]/g, ""))}
          inputMode="numeric"
          hint="Credits · 仅用于本次任务准入，不是账户余额"
        />

        {offline && <OfflineNotice />}
        {rejectedReason && <Text style={styles.error}>{rejectedReason}</Text>}
      </ScrollView>

      <View style={styles.footer}>
        <ActionButton
          label={submitting ? "正在提交…" : "开始执行"}
          onPress={submit}
          busy={submitting}
          block
          icon="send"
          accessibilityHint="提交前会先在本地保存请求身份"
        />
      </View>

      <DecisionSheet
        visible={uncertainId !== null}
        title="正在核实原请求"
        impact="提交回执未知，可能已被受理。我们按原请求编号查询，不会重复启动任务。"
        frozenSummary={[`请求编号：${uncertainId ?? ""}`, "同一意图只保留一个请求", "查询结果以服务端为准"]}
        confirm={{ label: "查询原请求", variant: "primary", onPress: reconcile, busy: submitting }}
        dismiss={{ label: "稍后再查", variant: "secondary", onPress: () => setUncertainId(null) }}
      />
    </KeyboardAvoidingView>
  );
}
