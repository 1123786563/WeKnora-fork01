// AUTO-PORTED from frontend/src/i18n/locales/*.ts -> resourceOrigin.{mine,tenant,space,shared}
// (badge text keys; tooltip keys stay in the Vue i18n files until a React consumer needs them).
// Keys are flattened with dot separators; values are byte-exact ports.
import type { Locale } from '../index.ts';

export const resourceOriginMessages: Record<Locale, Record<string, string>> = {
  "zh-CN": {"resourceOrigin.mine":"我创建","resourceOrigin.tenant":"本空间","resourceOrigin.space":"共享空间","resourceOrigin.shared":"外部共享"},
  "en-US": {"resourceOrigin.mine":"Mine","resourceOrigin.tenant":"Workspace","resourceOrigin.space":"Space","resourceOrigin.shared":"External"},
  "ja-JP": {"resourceOrigin.mine":"自分","resourceOrigin.tenant":"ワークスペース","resourceOrigin.space":"スペース","resourceOrigin.shared":"外部"},
  "ko-KR": {"resourceOrigin.mine":"내 생성","resourceOrigin.tenant":"워크스페이스","resourceOrigin.space":"스페이스","resourceOrigin.shared":"외부 공유"},
  "ru-RU": {"resourceOrigin.mine":"Мои","resourceOrigin.tenant":"Рабочая область","resourceOrigin.space":"Пространство","resourceOrigin.shared":"Внешнее"},
};
