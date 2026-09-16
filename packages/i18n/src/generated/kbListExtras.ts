// AUTO-PORTED (scripts/parity/backfill-i18n-keys.mjs) from
// frontend/src/i18n/locales/*.ts -> common.noMoreData (FAQ/KB list end-of-data hint). knowledgeBase.infoCard.{myRole,chunkCount,hitCount} requested by the slice do NOT exist in the Vue files (closest: knowledgeBase.accessInfo.myRole / knowledgeBase.chunkCount) and were NOT invented.
// Keys are flattened with dot separators; values are byte-exact ports.
import type { Locale } from '../index.ts';

export const kbListExtrasMessages: Record<Locale, Record<string, string>> = {
  "zh-CN": {"common.noMoreData":"已加载全部内容"},
  "en-US": {"common.noMoreData":"All content loaded"},
  "ja-JP": {"common.noMoreData":"すべての内容を読み込みました"},
  "ko-KR": {"common.noMoreData":"모든 내용을 로드했습니다"},
  "ru-RU": {"common.noMoreData":"Весь контент загружен"},
};
