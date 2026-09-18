import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  FlatList,
  KeyboardAvoidingView,
  Platform,
  StyleSheet,
  Text,
  TextInput,
  View,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { StateView } from "@/components/StateView";
import type { SessionWire } from "@/contracts/auth";

// M07 对话（02 规格）：会话上下文来自 GET /sessions；消息读取端点尚未联调——
// 消息区按真实数据渲染（当前为空态），发送不假装成功，仅显示等待联调的持久提示行。

interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  text: string;
}

/** 距底部小于该值视为"接近底部"，此时内容变化才自动滚底 */
const NEAR_BOTTOM_THRESHOLD = 80;

export default function ConversationScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{ sessionId: string }>();
  const sessionId = typeof params.sessionId === "string" ? params.sessionId : "";

  const [session, setSession] = useState<SessionWire | null>(null);
  const [agentName, setAgentName] = useState<string | null>(null);
  const [view, setView] = useState<"loading" | "ready" | "error">("loading");
  const [messages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [sendPendingNote, setSendPendingNote] = useState(false);

  const listRef = useRef<FlatList<ChatMessage> | null>(null);
  const nearBottomRef = useRef(true);

  const load = useCallback(async () => {
    if (!sessionId) {
      setView("ready");
      return;
    }
    setView("loading");
    try {
      const res = await app.api.sessions({ page: 1, page_size: 50 });
      const found = res.sessions.find((s) => s.id === sessionId) ?? null;
      setSession(found);
      setView("ready");
      if (found?.agent_id) {
        // Agent 名尽力而为：目录读取失败时保持空
        try {
          const agents = await app.api.agents();
          setAgentName(agents.find((a) => a.id === found.agent_id)?.name ?? null);
        } catch {
          setAgentName(null);
        }
      }
    } catch {
      // 列表读取失败但路由参数仍有效：直接按参数渲染（不阻塞会话视图）
      setSession(null);
      setView("ready");
    }
  }, [app, sessionId]);

  useEffect(() => {
    void load();
  }, [load]);

  // 用户上翻时不强制滚底：仅当接近底部才自动跟随
  const handleScroll = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const { layoutMeasurement, contentOffset, contentSize } = e.nativeEvent;
    const distanceFromBottom = contentSize.height - layoutMeasurement.height - contentOffset.y;
    nearBottomRef.current = distanceFromBottom < NEAR_BOTTOM_THRESHOLD;
  };

  const handleContentSizeChange = () => {
    if (nearBottomRef.current) {
      listRef.current?.scrollToEnd({ animated: true });
    }
  };

  // 消息发送：当前无消息写入端点（后端联调中），不做本地假成功
  const handleSend = () => {
    if (!draft.trim()) return;
    setSendPendingNote(true);
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
      borderBottomWidth: 1,
      borderBottomColor: theme.c.line,
      backgroundColor: theme.c.bg,
    },
    headerTitle: {
      flex: 1,
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    headerSub: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.muted,
      marginTop: 1,
    },
    list: { paddingHorizontal: theme.space[20], paddingVertical: theme.space[16] },
    dateLine: {
      alignSelf: "center",
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
      marginBottom: theme.space[12],
    },
    composer: {
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      backgroundColor: theme.c.bg,
      paddingHorizontal: theme.space[12],
      paddingTop: theme.space[8],
      paddingBottom: insets.bottom + theme.space[8],
      gap: theme.space[4],
    },
    pendingNote: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["info-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    pendingNoteText: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.info,
    },
    composerRow: {
      flexDirection: "row",
      alignItems: "flex-end",
      gap: theme.space[4],
    },
    input: {
      flex: 1,
      minHeight: theme.size["icon-touch"],
      maxHeight: 120,
      borderRadius: theme.radius.control,
      borderWidth: 1,
      borderColor: theme.c["control-line"],
      backgroundColor: theme.c.surface,
      paddingHorizontal: theme.space[16],
      paddingVertical: theme.space[12],
      fontSize: theme.type.body.fontSize,
      lineHeight: theme.type.body.lineHeight,
      color: theme.c.ink,
      textAlignVertical: "center",
    },
  });

  const title = session?.title || "会话";
  const today = new Date();
  const dateLine = `${today.getFullYear()} 年 ${today.getMonth() + 1} 月 ${today.getDate()} 日 · 当前空间`;

  return (
    <KeyboardAvoidingView style={styles.wrap} behavior={Platform.OS === "ios" ? "padding" : undefined} keyboardVerticalOffset={0}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <View style={{ flex: 1 }}>
          <Text style={styles.headerTitle} numberOfLines={1} accessibilityRole="header">
            {title}
          </Text>
          {agentName && <Text style={styles.headerSub}>{agentName}</Text>}
        </View>
      </View>

      {view === "loading" ? (
        <StateView state={{ kind: "loading" }} />
      ) : view === "error" ? (
        <StateView state={{ kind: "error", retry: () => void load() }} />
      ) : (
        <FlatList
          ref={listRef}
          data={messages}
          inverted={false}
          keyExtractor={(m) => m.id}
          renderItem={({ item }) => (
            <Text style={{ color: theme.c.ink }}>{item.text}</Text>
          )}
          ListHeaderComponent={<Text style={styles.dateLine}>{dateLine}</Text>}
          ListEmptyComponent={
            <StateView
              state={{
                kind: "empty",
                title: "这里还没有消息",
                description: "会话消息的读取与发送正在等待后端联调。",
              }}
            />
          }
          onScroll={handleScroll}
          onContentSizeChange={handleContentSizeChange}
          scrollEventThrottle={16}
          contentContainerStyle={styles.list}
          keyboardShouldPersistTaps="handled"
        />
      )}

      <View style={styles.composer}>
        {sendPendingNote && (
          <View style={styles.pendingNote} accessibilityRole="alert">
            <Icon name="info" size={16} color={theme.c.info} />
            <Text style={styles.pendingNoteText}>发送能力等待后端联调</Text>
          </View>
        )}
        <View style={styles.composerRow}>
          <TextInput
            value={draft}
            onChangeText={setDraft}
            placeholder="输入消息…"
            placeholderTextColor={theme.c.subtle}
            multiline
            style={styles.input}
            accessibilityLabel="输入消息"
            testID="conversation-input"
          />
          <IconButton name="send" label="发送" onPress={handleSend} testID="conversation-send" />
          <IconButton name="mic" label="语音输入" onPress={() => router.push("/voice")} testID="conversation-mic" />
        </View>
      </View>
    </KeyboardAvoidingView>
  );
}
