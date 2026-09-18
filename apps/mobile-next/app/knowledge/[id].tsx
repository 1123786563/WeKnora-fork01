import React, { useCallback, useEffect, useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { ActionButton } from "@/components/ActionButton";
import { StatusBadge } from "@/components/StatusBadge";
import { StateView } from "@/components/StateView";
import { SectionHeader } from "@/components/SearchField";
import type { BadgeSpec } from "@/components/StatusBadge";
import type { KnowledgeBaseWire } from "@/contracts/workbench";

// M12 知识详情：知识库摘要（文档数/索引状态/更新时间）、文档行、引用片段预览；
// 索引未就绪时不会参与检索；forbidden 不展示正文。
const FIXTURE_DOCS = ["第 38 周客户反馈.pdf", "产品访谈纪要.md", "体验问题处理记录.md"];
const FIXTURE_QUOTE = "“任务正在运行时，希望能看到当前阶段；离开页面后也能收到完成提醒。”";
const FIXTURE_QUOTE_SOURCE = "来源：第 38 周客户反馈 · 第 4 页";
const FIXTURE_KB: KnowledgeBaseWire = {
  id: "kb_demo_feedback",
  name: "客户反馈知识库",
  document_count: 24,
  status: "indexed",
  updated_at: "今天 09:16",
};

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; kb: KnowledgeBaseWire }
  | { kind: "empty" }
  | { kind: "error"; retry: () => void }
  | { kind: "forbidden" };

export default function KnowledgeDetailScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{ id?: string }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  const load = useCallback(async () => {
    if (!id) {
      setState({ kind: "empty" });
      return;
    }
    if (app.visualFixture) {
      setState({ kind: "ready", kb: FIXTURE_KB });
      return;
    }
    setState({ kind: "loading" });
    try {
      const list = await app.api.knowledgeBases();
      const kb = list.find((k) => k.id === id);
      if (!kb) {
        setState({ kind: "empty" });
        return;
      }
      setState({ kind: "ready", kb });
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "forbidden") {
        setState({ kind: "forbidden" });
        return;
      }
      setState({ kind: "error", retry: () => void load() });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app.visualFixture, id]);

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
      padding: theme.space[16],
      gap: theme.space[12],
    },
    headRow: { flexDirection: "row", alignItems: "center", gap: theme.space[12] },
    iconWrap: {
      width: 44,
      height: 44,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["info-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    kbName: { fontSize: theme.type.title.fontSize, lineHeight: theme.type.title.lineHeight, fontWeight: "600", color: theme.c.ink, flex: 1 },
    kbSub: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 2 },
    statStrip: {
      flexDirection: "row",
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      paddingTop: theme.space[12],
    },
    stat: { flex: 1, alignItems: "center", gap: 2 },
    statNum: { fontSize: theme.type.metric.fontSize, lineHeight: theme.type.metric.lineHeight, fontWeight: "700", color: theme.c.ink },
    statLabel: { fontSize: theme.type.caption.fontSize, color: theme.c.muted },
    notice: {
      marginTop: theme.space[16],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
      flexDirection: "row",
      gap: theme.space[8],
      alignItems: "flex-start",
    },
    noticeText: { flex: 1, color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    warningNotice: { backgroundColor: theme.c["warning-soft"] },
    warningNoticeText: { color: theme.c.warning },
    section: { marginTop: theme.space[24] },
    docRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      marginBottom: theme.space[12],
    },
    docIconWrap: {
      width: 36,
      height: 36,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["surface-alt"],
      alignItems: "center",
      justifyContent: "center",
    },
    docName: { fontSize: theme.type["body-sm"].fontSize + 1, color: theme.c.ink, fontWeight: "600" },
    quote: {
      backgroundColor: theme.c["surface-alt"],
      borderRadius: theme.radius.card,
      padding: theme.space[16],
    },
    quoteText: { fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight, color: theme.c.ink },
    quoteSource: { fontSize: theme.type.caption.fontSize, color: theme.c.subtle, marginTop: theme.space[8] },
    hintLine: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    footer: {
      padding: theme.space[20],
      paddingBottom: insets.bottom + theme.space[12],
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      backgroundColor: theme.c.bg,
    },
  });

  const badgeFor = (status: string): BadgeSpec => {
    if (status === "indexed") return { domain: "run", label: "已就绪", tone: "success" };
    if (status === "processing") return { domain: "run", label: "索引中", tone: "progress" };
    return { domain: "run", label: "等待同步", tone: "unknown", unknown: true };
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle}>知识详情</Text>
      </View>

      {state.kind === "loading" && <StateView state={{ kind: "loading" }} />}
      {state.kind === "forbidden" && <StateView state={{ kind: "forbidden" }} />}
      {state.kind === "error" && <StateView state={{ kind: "error", retry: state.retry }} />}
      {state.kind === "empty" && (
        <StateView state={{ kind: "empty", title: "知识库不存在", description: "资源不存在或你已无访问权限。重新同步后再尝试。" }} />
      )}

      {state.kind === "ready" && (
        <ScrollView contentContainerStyle={styles.scroll}>
          <View style={styles.card}>
            <View style={styles.headRow}>
              <View style={styles.iconWrap}>
                <Icon name="book" size={22} color={theme.c.info} />
              </View>
              <View style={{ flex: 1 }}>
                <Text style={styles.kbName} numberOfLines={2}>
                  {state.kb.name}
                </Text>
                <Text style={styles.kbSub}>{state.kb.updated_at ? `更新于 ${state.kb.updated_at}` : "更新时间待同步"}</Text>
              </View>
            </View>
            <View style={styles.statStrip}>
              <View style={styles.stat}>
                <Text style={styles.statNum}>{state.kb.document_count ?? "—"}</Text>
                <Text style={styles.statLabel}>文档</Text>
              </View>
              <View style={styles.stat}>
                <View style={{ minHeight: theme.type.metric.lineHeight, justifyContent: "center" }}>
                  <StatusBadge {...badgeFor(state.kb.status)} />
                </View>
                <Text style={styles.statLabel}>索引状态</Text>
              </View>
            </View>
          </View>

          {state.kb.status !== "indexed" && (
            <View style={[styles.notice, styles.warningNotice]}>
              <Icon name="info" size={16} color={theme.c.warning} />
              <Text style={[styles.noticeText, styles.warningNoticeText]}>索引未就绪时不会参与检索。</Text>
            </View>
          )}
          <View style={styles.notice}>
            <Icon name="shield" size={16} color={theme.c.brand} />
            <Text style={styles.noticeText}>只会检索你有权访问的内容。知识使用权限不等于文档管理权限。</Text>
          </View>

          <View style={styles.section}>
            <SectionHeader title="最近文档" />
            {app.visualFixture ? (
              FIXTURE_DOCS.map((doc) => (
                <View key={doc} style={styles.docRow}>
                  <View style={styles.docIconWrap}>
                    <Icon name="file" size={18} color={theme.c.muted} />
                  </View>
                  <Text style={styles.docName} numberOfLines={1}>
                    {doc}
                  </Text>
                </View>
              ))
            ) : (
              <Text style={styles.hintLine}>文档明细随索引服务同步，当前显示知识库汇总。</Text>
            )}
          </View>

          <View style={styles.section}>
            <SectionHeader title="引用预览" />
            <View style={styles.quote}>
              {app.visualFixture ? (
                <>
                  <Text style={styles.quoteText}>{FIXTURE_QUOTE}</Text>
                  <Text style={styles.quoteSource}>{FIXTURE_QUOTE_SOURCE}</Text>
                </>
              ) : (
                <Text style={styles.quoteText}>引用片段预览需要部署的检索服务（能力待接入）。</Text>
              )}
            </View>
          </View>
        </ScrollView>
      )}

      {state.kind === "ready" && (
        <View style={styles.footer}>
          <ActionButton label="用此知识提问" icon="spark" block onPress={() => router.push("/new-task")} accessibilityHint="跳转到新建任务，并引用此知识库" />
        </View>
      )}
    </View>
  );
}
