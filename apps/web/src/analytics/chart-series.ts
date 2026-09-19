// SP11 Task 9 fix round 1 — chart series → i18n label key mapping.
// recharts legend/tooltip names come from the wire data fields, but the i18n
// package's leaf-key convention is camelCase (analytics.uniqueUsers), so the
// snake_case field unique_users must map explicitly. Centralizing the table
// keeps the page free of composed keys (`analytics.${key}`) and gives the
// key-resolution test a single importable contract.
export interface ChartSeries {
  /** Wire data field rendered by the Line/Bar. */
  dataKey: string;
  /** i18n leaf key resolved via formatMessage for the legend name. */
  labelKey: string;
  /** Series stroke/fill color. */
  color: string;
}

/** Query trend: queries rides the accent green, feedback splits blue/red. */
export const TREND_SERIES: ChartSeries[] = [
  { dataKey: 'queries', labelKey: 'analytics.queries', color: '#07c05f' },
  { dataKey: 'likes', labelKey: 'analytics.likes', color: '#2e6de6' },
  { dataKey: 'dislikes', labelKey: 'analytics.dislikes', color: '#d54941' },
];

/** Agent usage: messages vs unique users (snake_case field → camelCase key). */
export const AGENT_SERIES: ChartSeries[] = [
  { dataKey: 'messages', labelKey: 'analytics.messages', color: '#07c05f' },
  { dataKey: 'unique_users', labelKey: 'analytics.uniqueUsers', color: '#7c4dff' },
];
