// SP14 Task 1 — settings general 分区顶部套餐卡片文案（billing.*）。
// Fresh translations (no Vue counterpart): carried by all five locales like
// the other React-side additions (SP12 usage pattern). 金额单位为微积分，
// 与 settings.usage.availableLabel/heldLabel 口径一致。
import type { Locale } from '../index.ts';

export const billingMessages: Record<Locale, Record<string, string>> = {
  "zh-CN": {"billing.cardTitle":"套餐与额度","billing.plan":"当前套餐","billing.paidUntil":"有效期至","billing.available":"可用余量（微积分）","billing.held":"冻结中（微积分）","billing.upgrade":"升级 / 续费","billing.noExpiry":"无固定到期（未订阅）"},
  "en-US": {"billing.cardTitle":"Plan & credits","billing.plan":"Plan","billing.paidUntil":"Valid until","billing.available":"Available (microcredits)","billing.held":"Held (microcredits)","billing.upgrade":"Upgrade / Renew","billing.noExpiry":"No fixed expiry (not subscribed)"},
  "ja-JP": {"billing.cardTitle":"プランとクレジット","billing.plan":"現在のプラン","billing.paidUntil":"有効期限","billing.available":"利用可能（マイクロクレジット）","billing.held":"保留中（マイクロクレジット）","billing.upgrade":"アップグレード / 更新","billing.noExpiry":"固定期限なし（未購入）"},
  "ko-KR": {"billing.cardTitle":"플랜 및 크레딧","billing.plan":"현재 플랜","billing.paidUntil":"유효 기간","billing.available":"사용 가능 (마이크로크레딧)","billing.held":"보류 중 (마이크로크레딧)","billing.upgrade":"업그레이드 / 갱신","billing.noExpiry":"고정 만료 없음(미구독)"},
  "ru-RU": {"billing.cardTitle":"Тариф и кредиты","billing.plan":"Текущий тариф","billing.paidUntil":"Действует до","billing.available":"Доступно (микрокредиты)","billing.held":"Заблокировано (микрокредиты)","billing.upgrade":"Улучшить / продлить","billing.noExpiry":"Без фиксированного срока (без подписки)"},
};
