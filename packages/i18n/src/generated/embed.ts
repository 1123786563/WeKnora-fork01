// Ported verbatim from frontend/src/i18n/embed.ts -> chat block (visitor subset) as embed.* keys.
// Keys are flattened with dot separators; values are byte-exact ports.
import type { Locale } from '../index.ts';

export const embedMessages: Record<Locale, Record<string, string>> = {
  "zh-CN": {"embed.newChat":"新对话","embed.inputPlaceholder":"请输入您的消息...","embed.send":"发送","embed.loading":"加载中...","embed.noMessages":"暂无消息","embed.suggestedQuestions":"你可以这样问我","embed.referencesTitle":"参考了{count}个相关内容","embed.referenceChunkCount":"{count}个片段","embed.imageReadFailed":"读取图片失败"},
  "en-US": {"embed.newChat":"New Chat","embed.inputPlaceholder":"Enter your message...","embed.send":"Send","embed.loading":"Loading...","embed.noMessages":"No messages","embed.suggestedQuestions":"You can ask me","embed.referencesTitle":"Referenced {count} related item(s)","embed.referenceChunkCount":"{count} chunk(s)","embed.imageReadFailed":"Failed to read image"},
  "ja-JP": {"embed.newChat":"新しいチャット","embed.inputPlaceholder":"メッセージを入力してください...","embed.send":"送信","embed.loading":"読み込み中...","embed.noMessages":"メッセージはありません","embed.suggestedQuestions":"こんな質問ができます","embed.referencesTitle":"関連する内容を{count}件参照しました","embed.referenceChunkCount":"{count}件のチャンク","embed.imageReadFailed":"画像の読み込みに失敗しました"},
  "ko-KR": {"embed.newChat":"New Chat","embed.inputPlaceholder":"Enter your message...","embed.send":"Send","embed.loading":"Loading...","embed.noMessages":"No messages","embed.suggestedQuestions":"You can ask me","embed.referencesTitle":"Referenced {count} related item(s)","embed.referenceChunkCount":"{count} chunk(s)","embed.imageReadFailed":"Failed to read image"},
  "ru-RU": {"embed.newChat":"New Chat","embed.inputPlaceholder":"Enter your message...","embed.send":"Send","embed.loading":"Loading...","embed.noMessages":"No messages","embed.suggestedQuestions":"You can ask me","embed.referencesTitle":"Referenced {count} related item(s)","embed.referenceChunkCount":"{count} chunk(s)","embed.imageReadFailed":"Failed to read image"},
};
