// SP14 Task 1 — settings general 分区顶部套餐卡片文案（billing.*）。
// Fresh translations (no Vue counterpart): carried by all five locales like
// the other React-side additions (SP12 usage pattern). 合同修复后卡片只保留
// 套餐+有效期（+升级按钮）：额度三元组（available/held）后端无端点提供，
// 已删键并新增 billing.baseTier（base_tier 空间无 base_tier_key 时的回退）。
import type { Locale } from '../index.ts';

export const billingMessages: Record<Locale, Record<string, string>> = {
  "zh-CN": {"billing.cardTitle":"套餐与额度","billing.plan":"当前套餐","billing.paidUntil":"有效期至","billing.upgrade":"升级 / 续费","billing.noExpiry":"无固定到期（未订阅）","billing.baseTier":"基础版"},
  "en-US": {"billing.cardTitle":"Plan & credits","billing.plan":"Plan","billing.paidUntil":"Valid until","billing.upgrade":"Upgrade / Renew","billing.noExpiry":"No fixed expiry (not subscribed)","billing.baseTier":"Base tier"},
  "ja-JP": {"billing.cardTitle":"プランとクレジット","billing.plan":"現在のプラン","billing.paidUntil":"有効期限","billing.upgrade":"アップグレード / 更新","billing.noExpiry":"固定期限なし（未購入）","billing.baseTier":"ベーシックプラン"},
  "ko-KR": {"billing.cardTitle":"플랜 및 크레딧","billing.plan":"현재 플랜","billing.paidUntil":"유효 기간","billing.upgrade":"업그레이드 / 갱신","billing.noExpiry":"고정 만료 없음(미구독)","billing.baseTier":"베이직 플랜"},
  "ru-RU": {"billing.cardTitle":"Тариф и кредиты","billing.plan":"Текущий тариф","billing.paidUntil":"Действует до","billing.upgrade":"Улучшить / продлить","billing.noExpiry":"Без фиксированного срока (без подписки)","billing.baseTier":"Базовый тариф"},
};
