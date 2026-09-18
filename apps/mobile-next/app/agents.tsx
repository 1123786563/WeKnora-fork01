import React, { useCallback, useEffect, useRef, useState } from "react";
import { FlatList, Pressable, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { FilterChips, type FilterChip } from "@/components/FilterChips";
import { SearchField } from "@/components/SearchField";
import { StateView } from "@/components/StateView";
import type { AgentWire } from "@/contracts/auth";
import { agentSelection } from "@/features/workbench/agentSelection";

// M06 Agent 选择（02 规格）：GET /agents 目录；搜索 + 分类筛选；
// 选中后写入模块级 agentSelection 并返回（M05 草稿回读）。
// 说明（D-06）：agent wire 无分类字段——通用/知识/专业由名称与描述关键字做客户端归类，
// 与服务端能力目录对齐需等后端补充字段。

const SEARCH_DEBOUNCE_MS = 300;

const FILTER_CHIPS: FilterChip[] = [
  { key: "all", label: "全部" },
  { key: "general", label: "通用" },
  { key: "knowledge", label: "知识" },
  { key: "special", label: "专业" },
];

type AgentCategory = "general" | "knowledge" | "special";

function categoryOf(a: AgentWire): AgentCategory {
  const text = `${a.name} ${a.description}`.toLowerCase();
  if (/知识|knowledge|检索|研究|research/.test(text)) return "knowledge";
  if (/专业|工程|代码|开发|engineer|code|dev|数据/.test(text)) return "special";
  return "general";
}

export default function AgentsScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [filter, setFilter] = useState("all");
  const [agents, setAgents] = useState<AgentWire[]>([]);
  const [view, setView] = useState<"loading" | "ready" | "forbidden" | "error">("loading");

  const genRef = useRef<number>(-1);

  useEffect(() => {
    const t = setTimeout(() => setKeyword(keywordInput.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [keywordInput]);

  const load = useCallback(async () => {
    const gen = app.scope.generation?.value ?? 0;
    genRef.current = gen;
    setView("loading");
    try {
      const list = await app.api.agents();
      const currentGen = app.scope.generation?.value ?? 0;
      if (genRef.current !== currentGen || gen !== currentGen) return;
      setAgents(list);
      setView("ready");
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "forbidden") setView("forbidden");
      else setView("error");
    }
  }, [app]);

  useEffect(() => {
    void load();
  }, [load]);

  const choose = (a: AgentWire) => {
    agentSelection.current = { id: a.id, name: a.name };
    router.back();
  };

  const filtered = agents.filter((a) => {
    if (filter !== "all" && categoryOf(a) !== filter) return false;
    if (!keyword) return true;
    return `${a.name} ${a.description}`.toLowerCase().includes(keyword.toLowerCase());
  });

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
      fontSize: theme.type.title.fontSize,
      lineHeight: theme.type.title.lineHeight,
      fontWeight: theme.type.title.fontWeight as "700",
      color: theme.c.ink,
    },
    filters: { paddingHorizontal: theme.space[20], gap: theme.space[12], paddingBottom: theme.space[12] },
    list: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[24] },
    separator: { height: theme.space[12] },
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
    },
    iconWrap: {
      width: 40,
      height: 40,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    col: { flex: 1, gap: 2 },
    nameRow: { flexDirection: "row", alignItems: "center", gap: theme.space[8] },
    name: {
      fontSize: theme.type["body-sm"].fontSize + 2,
      lineHeight: theme.type["body-sm"].lineHeight + 2,
      fontWeight: "600",
      color: theme.c.ink,
    },
    desc: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
    },
    builtinBadge: {
      borderRadius: theme.radius.pill,
      backgroundColor: theme.c["surface-alt"],
      paddingHorizontal: theme.space[8],
      paddingVertical: 2,
    },
    builtinText: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      fontWeight: "600",
      color: theme.c.muted,
    },
    notice: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
      marginHorizontal: theme.space[20],
      marginBottom: theme.space[24],
    },
    noticeText: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"],
    },
  });

  const renderEmpty = () => {
    if (view === "loading") return <StateView state={{ kind: "loading" }} />;
    if (view === "forbidden") return <StateView state={{ kind: "forbidden" }} />;
    if (view === "error") return <StateView state={{ kind: "error", retry: () => void load() }} />;
    return (
      <StateView
        state={{
          kind: "empty",
          title: keyword ? "没有匹配的 Agent" : "当前空间还没有可用 Agent",
          description: keyword ? "换个关键词或筛选条件试试。" : "能力目录来自当前空间。",
        }}
      />
    );
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle} accessibilityRole="header">
          选择 Agent
        </Text>
      </View>
      <View style={styles.filters}>
        <SearchField value={keywordInput} onChangeText={setKeywordInput} placeholder="搜索助手" testID="agents-search" />
        <FilterChips chips={FILTER_CHIPS} selected={filter} onSelect={(k) => setFilter(k)} testID="agents-filter" />
      </View>
      <FlatList
        data={filtered}
        keyExtractor={(a) => a.id}
        renderItem={({ item }) => (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={`选择 Agent ${item.name}`}
            onPress={() => choose(item)}
            testID={`agent-card-${item.id}`}
            style={({ pressed }) => [styles.card, pressed && { opacity: 0.8 }]}
          >
            <View style={styles.iconWrap}>
              <Icon name="spark" size={20} color={theme.c.brand} />
            </View>
            <View style={styles.col}>
              <View style={styles.nameRow}>
                <Text style={styles.name} numberOfLines={1}>
                  {item.name}
                </Text>
                {item.builtin && (
                  <View style={styles.builtinBadge}>
                    <Text style={styles.builtinText}>内置</Text>
                  </View>
                )}
              </View>
              {item.description ? (
                <Text style={styles.desc} numberOfLines={2}>
                  {item.description}
                </Text>
              ) : null}
            </View>
            <Icon name="chevron" size={16} color={theme.c.subtle} />
          </Pressable>
        )}
        ItemSeparatorComponent={() => <View style={styles.separator} />}
        ListEmptyComponent={renderEmpty}
        contentContainerStyle={styles.list}
        keyboardShouldPersistTaps="handled"
      />
      <View style={styles.notice} accessibilityRole="text">
        <Icon name="shield" size={16} color={theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"]} />
        <Text style={styles.noticeText}>能力来自当前空间。未就绪的能力不会通过点击按钮被启用。</Text>
      </View>
    </View>
  );
}
