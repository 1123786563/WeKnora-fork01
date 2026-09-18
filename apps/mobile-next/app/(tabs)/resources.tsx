import React, { useCallback, useEffect, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import type { IconName } from "@/components/Icon";
import { SearchField, SectionHeader } from "@/components/SearchField";
import { FilterChips } from "@/components/FilterChips";
import { StateView } from "@/components/StateView";
import { ActionButton } from "@/components/ActionButton";
import type { ConnectionWire, KnowledgeBaseWire } from "@/contracts/workbench";

// M11 资源（tab 页，无返回键）：当前空间已授权的知识、连接与成果入口；
// 无权限的资源不出现在这里；数据走真实 API，visualFixture 为演示开关。
const RESOURCE_FILTERS = [
  { key: "all", label: "全部" },
  { key: "knowledge", label: "知识" },
  { key: "connector", label: "连接" },
  { key: "artifact", label: "成果" },
] as const;

const FIXTURE_KNOWLEDGE: KnowledgeBaseWire[] = [
  { id: "kb_demo_feedback", name: "客户反馈知识库", document_count: 24, status: "indexed", updated_at: "今天 09:16" },
];
const FIXTURE_CONNECTIONS: ConnectionWire[] = [
  { id: "conn_demo_feishu", provider: "飞书连接", owner_scope: "tenant", status: "active", scopes: ["read:wiki", "write:wiki"], account_label: "project-bot" },
];
const FIXTURE_ARTIFACT = { id: "artifact_demo_feedback", name: "客户反馈分析.md", version: "3", sizeLabel: "12 KB", sourceRun: "run_demo_feedback" };

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; knowledge: KnowledgeBaseWire[]; connections: ConnectionWire[] }
  | { kind: "error"; retry: () => void }
  | { kind: "forbidden" };

export default function ResourcesScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<string>("all");

  const load = useCallback(async () => {
    if (app.visualFixture) {
      setState({ kind: "ready", knowledge: FIXTURE_KNOWLEDGE, connections: FIXTURE_CONNECTIONS });
      return;
    }
    setState({ kind: "loading" });
    try {
      const [knowledge, connections] = await Promise.all([app.api.knowledgeBases(), app.api.connections()]);
      setState({ kind: "ready", knowledge, connections });
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
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    header: {
      paddingTop: insets.top + theme.space[12],
      paddingBottom: theme.space[16],
      paddingHorizontal: theme.space[20],
    },
    headerTitle: { fontSize: theme.type.subtitle.fontSize, lineHeight: theme.type.subtitle.lineHeight, fontWeight: "600", color: theme.c.ink },
    headerSub: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 2 },
    tools: { gap: theme.space[12], marginBottom: theme.space[16] },
    section: { marginBottom: theme.space[24] },
    row: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      marginBottom: theme.space[12],
    },
    iconWrap: {
      width: 40,
      height: 40,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    iconWrapAlt: { backgroundColor: theme.c["surface-alt"] },
    col: { flex: 1, gap: 2 },
    name: { fontSize: theme.type["body-sm"].fontSize + 1, lineHeight: theme.type["body-sm"].lineHeight, fontWeight: "600", color: theme.c.ink },
    meta: { fontSize: theme.type.caption.fontSize, lineHeight: theme.type.caption.lineHeight, color: theme.c.subtle },
    targetHint: { fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight, color: theme.c.muted, flex: 1 },
    sectionGap: { marginTop: theme.space[8] },
  });

  const match = (name: string) => name.toLowerCase().includes(query.trim().toLowerCase());

  const knowledgeRows =
    state.kind === "ready"
      ? state.knowledge.filter((k) => match(k.name))
      : [];
  const connectionRows =
    state.kind === "ready"
      ? state.connections.filter((c) => match(c.provider))
      : [];
  const artifactRows = app.visualFixture && match(FIXTURE_ARTIFACT.name) ? [FIXTURE_ARTIFACT] : [];

  const showKnowledge = filter === "all" || filter === "knowledge";
  const showConnector = filter === "all" || filter === "connector";
  const showArtifact = filter === "all" || filter === "artifact";
  const showTargetRow = filter === "all";

  const noData = state.kind === "ready" && state.knowledge.length === 0 && state.connections.length === 0 && artifactRows.length === 0;
  const noMatch =
    state.kind === "ready" &&
    !noData &&
    (showKnowledge ? knowledgeRows.length : 0) + (showConnector ? connectionRows.length : 0) + (showArtifact ? artifactRows.length : 0) ===
      0;

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>资源</Text>
        <Text style={styles.headerSub}>当前空间 · 已授权资源</Text>
      </View>
      <ScrollView contentContainerStyle={styles.scroll}>
        <View style={styles.tools}>
          <SearchField value={query} onChangeText={setQuery} placeholder="搜索当前空间资源" />
          <FilterChips chips={[...RESOURCE_FILTERS]} selected={filter} onSelect={setFilter} />
        </View>

        {state.kind === "loading" && <StateView state={{ kind: "loading" }} />}
        {state.kind === "forbidden" && <StateView state={{ kind: "forbidden" }} />}
        {state.kind === "error" && <StateView state={{ kind: "error", retry: state.retry }} />}
        {noData && <StateView state={{ kind: "empty", title: "这里还没有资源", description: "知识、连接与成果授权后，会出现在这里。" }} />}
        {noMatch && (
          <StateView state={{ kind: "empty", title: "没有找到资源", description: "换个词搜索。无权限的资源不会出现在这里。" }} />
        )}

        {state.kind === "ready" && !noData && !noMatch && (
          <>
            {showKnowledge && knowledgeRows.length > 0 && (
              <View style={styles.section}>
                <SectionHeader title={`知识 · ${knowledgeRows.length}`} />
                {knowledgeRows.map((k) => (
                  <ResourceRow
                    key={k.id}
                    icon="book"
                    name={k.name}
                    meta={`${k.document_count ?? "—"} 篇文档 · ${knowledgeStatusLabel(k.status)}`}
                    accessibilityLabel={`知识库 ${k.name}`}
                    onPress={() => router.push(`/knowledge/${k.id}`)}
                  />
                ))}
              </View>
            )}

            {showConnector && connectionRows.length > 0 && (
              <View style={styles.section}>
                <SectionHeader title={`连接 · ${connectionRows.length}`} />
                {connectionRows.map((c) => (
                  <ResourceRow
                    key={c.id}
                    icon="link"
                    name={c.provider}
                    meta={`${c.owner_scope === "personal" ? "个人连接" : c.owner_scope === "tenant" ? "空间连接" : "连接"} · ${connectionStatusLabel(c.status)}`}
                    accessibilityLabel={`连接 ${c.provider}`}
                    onPress={() => router.push(`/connections/${c.id}`)}
                  />
                ))}
              </View>
            )}

            {showArtifact && (
              <View style={styles.section}>
                <SectionHeader title="成果" />
                {artifactRows.length > 0 ? (
                  artifactRows.map((a) => (
                    <ResourceRow
                      key={a.id}
                      icon="file"
                      name={a.name}
                      meta={`v${a.version} · ${a.sizeLabel}`}
                      accessibilityLabel={`成果 ${a.name}`}
                      onPress={() => router.push(`/artifacts/${a.id}?runId=${a.sourceRun}`)}
                    />
                  ))
                ) : (
                  <Text style={styles.targetHint}>成果随任务生成，可在对应的执行详情中查看。</Text>
                )}
              </View>
            )}

            {showTargetRow && (
              <View style={[styles.section, styles.sectionGap]}>
                <SectionHeader title="执行目标" />
                <Pressable
                  style={({ pressed }) => [styles.row, pressed && { opacity: 0.8 }]}
                  accessibilityRole="button"
                  accessibilityLabel="查看可用的执行目标"
                  onPress={() => router.push("/targets")}
                >
                  <View style={[styles.iconWrap, styles.iconWrapAlt]}>
                    <Icon name="monitor" size={20} color={theme.c.muted} />
                  </View>
                  <View style={styles.col}>
                    <Text style={styles.name}>平台执行与受控节点</Text>
                    <Text style={styles.meta}>任务在哪里运行，由服务端受权引用决定</Text>
                  </View>
                  <Icon name="chevron" size={16} color={theme.c.subtle} />
                </Pressable>
                <ActionButton
                  label="查看可用的执行目标"
                  variant="secondary"
                  icon="monitor"
                  block
                  onPress={() => router.push("/targets")}
                />
              </View>
            )}
          </>
        )}
      </ScrollView>
    </View>
  );

  function ResourceRow({
    icon,
    name,
    meta,
    accessibilityLabel,
    onPress,
  }: {
    icon: IconName;
    name: string;
    meta: string;
    accessibilityLabel: string;
    onPress: () => void;
  }) {
    return (
      <Pressable
        style={({ pressed }) => [styles.row, pressed && { opacity: 0.8 }]}
        accessibilityRole="button"
        accessibilityLabel={accessibilityLabel}
        onPress={onPress}
      >
        <View style={styles.iconWrap}>
          <Icon name={icon} size={20} color={theme.c.brand} />
        </View>
        <View style={styles.col}>
          <Text style={styles.name} numberOfLines={1}>
            {name}
          </Text>
          <Text style={styles.meta} numberOfLines={1}>
            {meta}
          </Text>
        </View>
        <Icon name="chevron" size={16} color={theme.c.subtle} />
      </Pressable>
    );
  }
}

function knowledgeStatusLabel(status: string): string {
  if (status === "indexed") return "已就绪";
  if (status === "processing") return "索引中";
  return "等待同步";
}

function connectionStatusLabel(status: string): string {
  if (status === "active") return "可用";
  if (status === "expired") return "授权过期";
  return "等待同步";
}
