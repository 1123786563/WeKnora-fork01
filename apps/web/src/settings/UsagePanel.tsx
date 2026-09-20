// SP12 Task 7 — settings 用量分区面板（section=usage，用户级）。
// 数据来自 client.usage.my（GET /api/v1/usage/me，Viewer+）；套餐信息卡片来自
// client.commercial.summary（真实形状：subscription|base_tier），summary 不可用时
// catch 静默隐藏整卡，分区其余内容照常。日期窗口复用 analytics-range.ts 的
// UTC 日界契约（defaultAnalyticsRange/clampAnalyticsRange），应用按钮提交后才发请求。
import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { CommercialSummary, UsageRow } from '@weknora/contracts';
import { formatMessage, type Locale } from '@weknora/i18n';
import { Button, Card, Status } from '@weknora/ui';
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

// 与 AnalyticsPage 相同的日期输入样式（analytics-range 选区契约共用）。
const USAGE_DATE_INPUT = 'box-border h-[32px] rounded-[6px] border border-[#e7e7ea] bg-surface px-[8px] font-[inherit] text-[13px] text-[rgba(23,26,29,0.92)] focus:border-accent focus:outline-none';
const USAGE_TABLE = 'w-full border-collapse text-[13px]';
const USAGE_TABLE_CELL = 'border-b border-[#eef1f5] px-[10px] py-[8px] text-left';
const USAGE_NUMBER_CELL = USAGE_TABLE_CELL + ' text-right tabular-nums';

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
    <div data-testid="usage-panel" className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-[8px]">
        <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
          {t('settings.usage.rangeFrom')}
          <input type="date" className={USAGE_DATE_INPUT} value={fromInput} onChange={(event) => setFromInput(event.target.value)} />
        </label>
        <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
          {t('settings.usage.rangeTo')}
          <input type="date" className={USAGE_DATE_INPUT} value={toInput} onChange={(event) => setToInput(event.target.value)} />
        </label>
        <Button type="button" onClick={applyRange}>{t('settings.usage.apply')}</Button>
      </div>

      {budget && budgetCopy ? (
        <Card data-testid="usage-budget-card" className="flex flex-wrap gap-x-[24px] gap-y-[6px]">
          <h3 className="w-full m-0 text-[15px] font-semibold text-[rgba(23,26,29,0.92)]">{t('settings.usage.budgetTitle')}</h3>
          <span className="text-[13px]"><span className="text-[rgba(23,26,29,0.6)]">{t('settings.usage.planLabel')}：</span>{budgetCopy.plan}</span>
          <span className="text-[13px]"><span className="text-[rgba(23,26,29,0.6)]">{t('settings.usage.paidUntilLabel')}：</span>{budgetCopy.paidUntil}</span>
        </Card>
      ) : null}

      {loading ? (
        <Status>{t('common.loading')}</Status>
      ) : error ? (
        <div role="alert" className="flex flex-wrap items-center gap-2">
          <Status tone="error">{error}</Status>
          <Button type="button" onClick={() => void load()}>{t('common.retry')}</Button>
        </div>
      ) : modelRows.length === 0 ? (
        <Status>{t('common.empty')}</Status>
      ) : (
        <table className={USAGE_TABLE} data-testid="usage-model-table">
          <thead>
            <tr className="border-b border-[#e7e7ea]">
              <th className={USAGE_TABLE_CELL + ' font-semibold'}>{t('settings.usage.colModel')}</th>
              <th className={USAGE_NUMBER_CELL + ' font-semibold'}>{t('settings.usage.colInput')}</th>
              <th className={USAGE_NUMBER_CELL + ' font-semibold'}>{t('settings.usage.colOutput')}</th>
              <th className={USAGE_NUMBER_CELL + ' font-semibold'}>{t('settings.usage.colCache')}</th>
              <th className={USAGE_NUMBER_CELL + ' font-semibold'}>{t('settings.usage.colCost')}</th>
            </tr>
          </thead>
          <tbody>
            {modelRows.map((row) => (
              <tr key={row.model}>
                <td className={USAGE_TABLE_CELL}>{row.model}</td>
                <td className={USAGE_NUMBER_CELL}>{number(row.input)}</td>
                <td className={USAGE_NUMBER_CELL}>{number(row.output)}</td>
                <td className={USAGE_NUMBER_CELL}>{number(row.cache)}</td>
                <td className={USAGE_NUMBER_CELL}>{number(row.cost)}</td>
              </tr>
            ))}
            <tr className="font-semibold">
              <td className={USAGE_TABLE_CELL}>{t('settings.usage.total')}</td>
              <td className={USAGE_NUMBER_CELL}>{number(totals.input)}</td>
              <td className={USAGE_NUMBER_CELL}>{number(totals.output)}</td>
              <td className={USAGE_NUMBER_CELL}>{number(totals.cache)}</td>
              <td className={USAGE_NUMBER_CELL}>{number(totals.cost)}</td>
            </tr>
          </tbody>
        </table>
      )}
    </div>
  );
}
