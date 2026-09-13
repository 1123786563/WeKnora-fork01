// O04 craft usage panel: the session's usage aggregation and execution
// diagnostics, standalone by design (same adoption pattern as the C02
// interaction panel — the workbench wiring belongs to the integrator).
//
// The panel talks to the real O04 HTTP surface:
//
//   GET /api/v1/sessions/:sid/craft/usage
//
// Display rules mirrored from the server contract:
//   - unknown consumption is NEVER folded into zeros — the label always
//     states "N 次用量待核对" so an unobserved call stays reconcilable;
//   - the numbers carry an explicit as_of stamp and a refresh control: a
//     late usage correction arrives as a new ledger revision, and refreshing
//     re-reads the ledger and moves as_of forward;
//   - BYOK involvement is stated plainly (the space's own credentials bear
//     those model calls); nothing anywhere renders an amount — money comes
//     only from the commercial view.
import { useCallback, useEffect, useState } from 'react';
import {
  asOfLabel, fundingNote, usageLabel, usageRows,
  type UsageView, type UsageCallRow,
} from '@weknora/domain/craft/usage';

// ---------------------------------------------------------------------------
// Wire types (snake_case, server-derived identity only)
// ---------------------------------------------------------------------------

/** The wire aggregation exactly as the endpoint returns it (snake_case). */
export interface CraftUsageAggregateWire {
  known_calls: number;
  unknown_calls: number;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  funding: 'platform' | 'byok' | 'mixed';
}

/** One physical call exactly as the endpoint returns it (snake_case). */
export interface CraftUsageCallWire {
  call_id: string;
  attempt_id: string;
  run_id: string;
  runtime: 'main' | 'oc';
  delegation_id: string;
  model_id: string;
  funding: string;
  status: string;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
}

export interface CraftUsageRunView {
  run_id: string;
  status: string;
  failure_reason: string;
  created_at: string;
  updated_at: string;
}

export interface CraftUsageResidencyView {
  starts: number;
  stops: number;
  open_starts: number;
  dwell_seconds: number;
  storage_bytes_day: number;
}

export interface CraftUsageCheckView {
  version_id: string;
  name: string;
  status: string;
  detail: string;
}

export interface CraftUsageData {
  as_of: string;
  usage: CraftUsageAggregateWire;
  byok_model_borne_by_space: boolean;
  runs: CraftUsageRunView[];
  calls: CraftUsageCallWire[];
  sandbox_residency?: CraftUsageResidencyView;
  checks?: CraftUsageCheckView[];
}

export interface CraftUsageClient {
  read(signal?: AbortSignal): Promise<CraftUsageData>;
}

async function usageErrorText(response: Response): Promise<string> {
  try {
    return (await response.text()) || String(response.status);
  } catch {
    return String(response.status);
  }
}

/** Builds the O04 usage client bound to one session over an injectable fetch. */
export function createSessionCraftUsageClient(
  fetchImpl: typeof fetch,
  sessionId: string,
  baseUrl = '',
): CraftUsageClient {
  const base = baseUrl.replace(/\/$/, '');
  return {
    read: async (signal) => {
      const response = await fetchImpl(
        base + '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/usage',
        { signal },
      );
      if (!response.ok) throw new Error('craft usage HTTP ' + response.status + ': ' + (await usageErrorText(response)));
      const body = (await response.json()) as { data?: CraftUsageData };
      if (!body.data) throw new Error('craft usage HTTP: empty payload');
      return body.data;
    },
  };
}

// ---------------------------------------------------------------------------
// Strings
// ---------------------------------------------------------------------------

export interface CraftUsageStrings {
  title: string;
  refresh: string;
  refreshing: string;
  loadFailed: string;
  emptyCalls: string;
  callsTitle: string;
  runsTitle: string;
  residencyTitle: string;
  residencyLine: (r: CraftUsageResidencyView) => string;
  checksTitle: string;
  failureAt: (reason: string) => string;
  tokensLine: (v: UsageView & { cached_tokens: number }) => string;
}

export const CRAFT_USAGE_STRINGS_ZH: CraftUsageStrings = {
  title: '用量与执行诊断',
  refresh: '刷新',
  refreshing: '刷新中…',
  loadFailed: '用量数据读取失败',
  emptyCalls: '暂无已记录的模型调用',
  callsTitle: '物理调用（主 / 子）',
  runsTitle: '运行',
  residencyTitle: '沙箱驻留',
  residencyLine: (r) =>
    '启动 ' + r.starts + ' 次 · 停止 ' + r.stops + ' 次' +
    (r.open_starts > 0 ? ' · 未配对启动 ' + r.open_starts + ' 次' : '') +
    ' · 累计驻留 ' + Math.round(r.dwell_seconds) + ' 秒',
  checksTitle: '检查',
  failureAt: (reason) => '失败原因：' + reason,
  tokensLine: (v) =>
    '输入 ' + v.inputTokens + ' · 输出 ' + v.outputTokens + ' · 缓存命中 ' + v.cached_tokens +
    '（缓存为输入的子集，不重复计）',
};

// ---------------------------------------------------------------------------
// Panel
// ---------------------------------------------------------------------------

export interface CraftUsagePanelProps {
  sessionId: string;
  client: CraftUsageClient;
  strings?: CraftUsageStrings;
}

export function CraftUsagePanel({ sessionId, client, strings = CRAFT_USAGE_STRINGS_ZH }: CraftUsagePanelProps) {
  const [data, setData] = useState<CraftUsageData | null>(null);
  const [error, setError] = useState('');
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      const next = await client.read(signal);
      setData(next);
      setError('');
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      setError(strings.loadFailed + ': ' + (err as Error).message);
    }
  }, [client, strings.loadFailed]);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load, sessionId]);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await load();
    } finally {
      setRefreshing(false);
    }
  }, [load]);

  if (error && !data) {
    return (
      <section className="wk-craft-usage" data-testid="craft-usage-error">
        <h3>{strings.title}</h3>
        <p role="alert">{error}</p>
      </section>
    );
  }
  if (!data) return null;

  const view: UsageView = {
    knownCalls: data.usage.known_calls,
    unknownCalls: data.usage.unknown_calls,
    inputTokens: data.usage.input_tokens,
    outputTokens: data.usage.output_tokens,
    funding: data.usage.funding,
  };
  const cachedTokens = data.usage.cached_tokens;
  const rows = usageRows(data.calls.map((call) => ({
    callId: call.call_id,
    attemptId: call.attempt_id,
    runtime: call.runtime,
    delegationId: call.delegation_id,
    modelId: call.model_id,
    funding: call.funding,
    status: call.status,
    inputTokens: call.input_tokens,
    outputTokens: call.output_tokens,
    cachedTokens: call.cached_tokens,
    observedAt: '',
  })));

  return (
    <section className="wk-craft-usage" data-testid="craft-usage-panel">
      <h3>{strings.title}</h3>
      <p data-testid="craft-usage-label">{usageLabel(view)}</p>
      {view.unknownCalls > 0 ? (
        <p data-testid="craft-usage-unknown-note">
          有 {view.unknownCalls} 次调用的用量未能观测（断流或取消），不会计入上方 token 数，待核对修正。
        </p>
      ) : null}
      <p data-testid="craft-usage-tokens">{strings.tokensLine({ ...view, cached_tokens: cachedTokens })}</p>
      {data.byok_model_borne_by_space ? (
        <p data-testid="craft-usage-byok">{fundingNote(view)}</p>
      ) : null}
      <p data-testid="craft-usage-asof">
        {asOfLabel(data.as_of)}{' '}
        <button type="button" onClick={() => void refresh()} disabled={refreshing}>
          {refreshing ? strings.refreshing : strings.refresh}
        </button>
      </p>

      <h4>{strings.callsTitle}</h4>
      {rows.length === 0 ? (
        <p data-testid="craft-usage-empty">{strings.emptyCalls}</p>
      ) : (
        <ul data-testid="craft-usage-calls">
          {rows.map((row, index) => <li key={index}>{row}</li>)}
        </ul>
      )}

      <h4>{strings.runsTitle}</h4>
      <ul data-testid="craft-usage-runs">
        {data.runs.map((run) => (
          <li key={run.run_id}>
            {run.run_id} · {run.status}
            {run.status === 'failed' && run.failure_reason ? ' · ' + strings.failureAt(run.failure_reason) : ''}
          </li>
        ))}
      </ul>

      {data.sandbox_residency ? (
        <>
          <h4>{strings.residencyTitle}</h4>
          <p data-testid="craft-usage-residency">{strings.residencyLine(data.sandbox_residency)}</p>
        </>
      ) : null}

      {data.checks && data.checks.length > 0 ? (
        <>
          <h4>{strings.checksTitle}</h4>
          <ul data-testid="craft-usage-checks">
            {data.checks.map((check) => (
              <li key={check.version_id + check.name}>
                {check.name} · {check.status} · {check.detail}
              </li>
            ))}
          </ul>
        </>
      ) : null}
    </section>
  );
}

// ---------------------------------------------------------------------------
// Mount-point API (integrator adoption)
// ---------------------------------------------------------------------------
// The workbench wiring belongs to the integrator (workbench.tsx is a parallel
// ownership file). Adoption is one block:
//
//   const usage = useMemo(
//     () => createSessionCraftUsageClient(authedFetch, sessionId, apiBaseUrl),
//     [authedFetch, sessionId, apiBaseUrl],
//   );
//   <CraftUsageMount sessionId={sessionId} client={usage} />

export type CraftUsageMountProps = CraftUsagePanelProps;

/** Stable mount-point alias for the panel (see the adoption snippet above). */
export function CraftUsageMount(props: CraftUsageMountProps) {
  return <CraftUsagePanel {...props} />;
}
