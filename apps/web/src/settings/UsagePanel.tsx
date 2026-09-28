// SP12 Task 7 — settings 用量分区面板（section=usage，用户级）。
// 数据来自 client.usage.my（GET /api/v1/usage/me，Viewer+）；套餐信息卡片来自
// client.commercial.summary（真实形状：subscription|base_tier），summary 不可用时
// catch 静默隐藏整卡，分区其余内容照常。日期窗口复用 analytics-range.ts 的
// UTC 日界契约（defaultAnalyticsRange/clampAnalyticsRange），应用按钮提交后才发请求。
import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { CommercialSummary, UsageRow } from '@weknora/contracts';
import { formatMessage, type Locale } from '@weknora/i18n';
import { Button as TButton } from 'tdesign-react';
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { clampAnalyticsRange, defaultAnalyticsRange, type AnalyticsDateRange } from '../analytics/analytics-range.ts';
import { formatBillingSummary } from './GeneralPreferencesPanel.tsx';

/** 按模型聚合后的一行：cache = cache_read + cache_write，cost 单位为微积分。 */
export interface UsageModelAggregate {
  readonly model: string;
  readonly input: number;
  readonly output: number;
  readonly cache: number;
  readonly cost: number;
}

/** items（多时间窗 × 多模型）→ 每模型一行，保持首次出现顺序。 */
export function aggregateByModel(items: readonly UsageRow[]): UsageModelAggregate[] {
  const byModel = new Map<string, UsageModelAggregate>();
  for (const item of items) {
    const current = byModel.get(item.model) ?? { model: item.model, input: 0, output: 0, cache: 0, cost: 0 };
    byModel.set(item.model, {
      model: item.model,
      input: current.input + item.input_tokens,
      output: current.output + item.output_tokens,
      cache: current.cache + item.cache_read_tokens + item.cache_write_tokens,
      cost: current.cost + item.cost_microcredits,
    });
  }
  return [...byModel.values()];
}

/** 合计行（model 置空，由渲染层填「合计」标签）。 */
export function usageTotals(rows: readonly UsageModelAggregate[]): UsageModelAggregate {
  return rows.reduce(
    (total, row) => ({
      model: '',
      input: total.input + row.input,
      output: total.output + row.output,
      cache: total.cache + row.cache,
      cost: total.cost + row.cost,
    }),
    { model: '', input: 0, output: 0, cache: 0, cost: 0 },
  );
}

/* S6 Tailwind 收编：USAGE_* 常量 → settings-wrapper.css .usage-* 规则。 */

function errorText(reason: unknown, fallback: string): string { return reason instanceof Error ? reason.message : fallback; }

export function UsagePanel({ client, locale = 'zh-CN' }: { client: WeKnoraClient; locale?: Locale }) {
  const t = (key: string): string => formatMessage(locale, key);
  const initialRange = useMemo(() => defaultAnalyticsRange(), []);
  const [range, setRange] = useState<AnalyticsDateRange>(initialRange);
  const [fromInput, setFromInput] = useState(initialRange.startTime);
  const [toInput, setToInput] = useState(initialRange.endTime);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [items, setItems] = useState<UsageRow[]>([]);
  const [budget, setBudget] = useState<CommercialSummary | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const result = await client.usage.my({ startTime: range.startTime, endTime: range.endTime });
      setItems(result.items);
    } catch (reason) {
      setItems([]);
      setError(errorText(reason, t('common.error')));
    } finally {
      setLoading(false);
    }
  }

  // 应用后的 range（而非草稿输入）是 effect 依赖：敲日期选择器不发请求。
  useEffect(() => {
    void load();
    /* eslint-disable-line react-hooks/exhaustive-deps */
  }, [client, range]);

  // 当前套餐信息与日期窗口无关，仅在挂载时拉取一次；commercial summary 不可用
  // （未部署 / 无权限）时静默隐藏整卡，不影响分区其余内容。
  useEffect(() => {
    let cancelled = false;
    client.commercial.summary()
      .then((summary) => { if (!cancelled) setBudget(summary); })
      .catch(() => { if (!cancelled) setBudget(null); });
    return () => { cancelled = true; };
  }, [client]);

  function applyRange() {
    const next = clampAnalyticsRange(fromInput, toInput);
    setFromInput(next.startTime);
    setToInput(next.endTime);
    setRange(next);
  }

  const modelRows = useMemo(() => aggregateByModel(items), [items]);
  const totals = useMemo(() => usageTotals(modelRows), [modelRows]);
  const number = (value: number): string => value.toLocaleString(locale);
  // 套餐卡片文案复用 SP14 general 卡片的 formatBillingSummary（真实形状：
  // plan_key / base_tier_key 回退 + paid_until 或 noExpiry）。
  const budgetCopy = budget ? formatBillingSummary(locale, budget) : null;

  return (
    <div data-testid="usage-panel" className="usage-panel">
      <div className="usage-filters">
        <label className="usage-filter">
          {t('settings.usage.rangeFrom')}
          <input type="date" className="usage-input" value={fromInput} onChange={(value) => setFromInput(String(value))} />
        </label>
        <label className="usage-filter">
          {t('settings.usage.rangeTo')}
          <input type="date" className="usage-input" value={toInput} onChange={(value) => setToInput(String(value))} />
        </label>
        <TButton type="button" onClick={applyRange}>{t('settings.usage.apply')}</TButton>
      </div>

      {budget && budgetCopy ? (
        <Card data-testid="usage-budget-card" className="usage-budget-card">
          <h3>{t('settings.usage.budgetTitle')}</h3>
          <span><span className="usage-budget-label">{t('settings.usage.planLabel')}：</span>{budgetCopy.plan}</span>
          <span><span className="usage-budget-label">{t('settings.usage.paidUntilLabel')}：</span>{budgetCopy.paidUntil}</span>
        </Card>
      ) : null}

      {loading ? (
        <Status>{t('common.loading')}</Status>
      ) : error ? (
        <div role="alert" className="usage-inline-error">
          <Status tone="error">{error}</Status>
          <TButton type="button" onClick={() => void load()}>{t('common.retry')}</TButton>
        </div>
      ) : modelRows.length === 0 ? (
        <Status>{t('common.empty')}</Status>
      ) : (
        <table className="usage-table" data-testid="usage-model-table">
          <thead>
            <tr>
              <th className="usage-th">{t('settings.usage.colModel')}</th>
              <th className="usage-th usage-th--num">{t('settings.usage.colInput')}</th>
              <th className="usage-th usage-th--num">{t('settings.usage.colOutput')}</th>
              <th className="usage-th usage-th--num">{t('settings.usage.colCache')}</th>
              <th className="usage-th usage-th--num">{t('settings.usage.colCost')}</th>
            </tr>
          </thead>
          <tbody>
            {modelRows.map((row) => (
              <tr key={row.model}>
                <td className="usage-td">{row.model}</td>
                <td className="usage-td usage-td--num">{number(row.input)}</td>
                <td className="usage-td usage-td--num">{number(row.output)}</td>
                <td className="usage-td usage-td--num">{number(row.cache)}</td>
                <td className="usage-td usage-td--num">{number(row.cost)}</td>
              </tr>
            ))}
            <tr className="usage-total-row">
              <td className="usage-td">{t('settings.usage.total')}</td>
              <td className="usage-td usage-td--num">{number(totals.input)}</td>
              <td className="usage-td usage-td--num">{number(totals.output)}</td>
              <td className="usage-td usage-td--num">{number(totals.cache)}</td>
              <td className="usage-td usage-td--num">{number(totals.cost)}</td>
            </tr>
          </tbody>
        </table>
      )}
    </div>
  );
}
