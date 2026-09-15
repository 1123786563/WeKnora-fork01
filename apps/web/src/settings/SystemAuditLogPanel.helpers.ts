const AUDIT_COLUMN_LABELS: Record<string, [string, string, string, string, string]> = {
  'zh-CN': ['时间', '操作者', '操作', '目标', '结果'],
  'en-US': ['Time', 'Actor', 'Action', 'Target', 'Outcome'],
  'ja-JP': ['時刻', '操作者', '操作', '対象', '結果'],
  'ko-KR': ['시간', '작업자', '작업', '대상', '결과'],
  'ru-RU': ['Время', 'Оператор', 'Действие', 'Цель', 'Результат'],
};

export function auditColumnLabels(locale: string): [string, string, string, string, string] {
  return AUDIT_COLUMN_LABELS[locale] ?? AUDIT_COLUMN_LABELS['zh-CN'];
}
