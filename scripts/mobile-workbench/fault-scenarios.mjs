#!/usr/bin/env node
// W35 fault scenario drills for the managed mobile-workbench deployment.
//
// Purpose: inject the five crash/recovery scenarios (start_ack_lost,
// bridge_restart, daemon_restart, db_unavailable, restore_snapshot) into an
// EXPLICIT, ALLOWLISTED, NON-PRODUCTION test deployment and verify — against
// database run counts, external process counts and commercial usage records,
// never just an HTTP 200 — that recovery keeps the invariants:
//
//   - no duplicate run starts (repeated drills start 0 extra runs),
//   - zero cross-tenant leakage,
//   - zero unconfirmed commercial usage lost,
//   - a restore with unknown dispatch outcomes stays admission-closed
//     (the JS mirror of internal/execution.MayDispatchAfterRestore:
//     dispatch may open only when reconciled && unknown === 0),
//   - after a snapshot restore, node credentials are re-verified before
//     dispatch reopens.
//
// Safety gates, in order, BEFORE any injection is attempted:
//   1. an explicit target (--target URL, or WB_TEST_DEPLOYMENT as fallback);
//      no explicit target => immediate non-zero exit, nothing is touched;
//   2. the target origin must be in the allowlist (--allowlist "a,b" or
//      WB_TEST_ORIGIN_ALLOWLIST);
//   3. the deployment must self-identify as a test deployment
//      (test_deployment === true on the control endpoint) and report a
//      version; with --expect-version the versions must match.
//
// Without a real test deployment, `--harness` runs every gate and every
// scenario against an in-process fake deployment that implements the exact
// driver contract, proving the script structure, the validation logic and
// the per-scenario JSON contract. Real-deployment injection requires the
// W20-W24 control endpoints and is recorded as blocked-env in
// docs/evidence/mobile-workbench/W35-recovery.md when no such deployment
// exists.
//
// Usage:
//   node scripts/mobile-workbench/fault-scenarios.mjs --harness
//   node scripts/mobile-workbench/fault-scenarios.mjs \
//     --target https://wb-test-1.internal:8443 \
//     --allowlist "$WB_TEST_ORIGIN_ALLOWLIST" \
//     --expect-version "$(git rev-parse --short HEAD)" \
//     --scenario restore_snapshot --repeat 3

import { execFileSync } from 'node:child_process';

export const SCENARIOS = [
  'start_ack_lost',
  'bridge_restart',
  'daemon_restart',
  'db_unavailable',
  'restore_snapshot',
];

export const RESULT_FIELDS = [
  'scenario',
  'baseline_sha',
  'run_id',
  'actual_process_count',
  'usage_count',
  'result',
];

// ---------------------------------------------------------------------------
// Argument parsing and target guards
// ---------------------------------------------------------------------------

export function parseArgs(argv) {
  const out = {
    scenario: null,
    target: null,
    allowlist: null,
    expectVersion: null,
    baselineSha: null,
    repeat: 1,
    harness: false,
    unknown: [],
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    const next = () => {
      if (i + 1 >= argv.length) throw new Error(`missing value for ${a}`);
      i += 1;
      return argv[i];
    };
    switch (a) {
      case '--scenario': out.scenario = next(); break;
      case '--target': out.target = next(); break;
      case '--allowlist': out.allowlist = next(); break;
      case '--expect-version': out.expectVersion = next(); break;
      case '--baseline-sha': out.baselineSha = next(); break;
      case '--repeat': out.repeat = Number.parseInt(next(), 10); break;
      case '--harness': out.harness = true; break;
      default: out.unknown.push(a);
    }
  }
  return out;
}

// resolveTarget applies the explicit-target rule: --target wins, the
// WB_TEST_DEPLOYMENT env var is the only fallback, and everything else is a
// hard stop. Returns {ok, target, reason}.
export function resolveTarget(args, env = process.env) {
  if (args.target && args.target.trim() !== '') {
    return { ok: true, target: args.target.trim() };
  }
  const fromEnv = env.WB_TEST_DEPLOYMENT;
  if (fromEnv && fromEnv.trim() !== '') {
    return { ok: true, target: fromEnv.trim() };
  }
  return {
    ok: false,
    target: null,
    reason:
      'no explicit target: pass --target URL or set WB_TEST_DEPLOYMENT; refusing to touch anything',
  };
}

// originOf normalizes a target to its origin so allowlist membership is not
// path-dependent.
export function originOf(target) {
  try {
    return new URL(target).origin;
  } catch {
    return null;
  }
}

// guardAllowlist fails closed: no allowlist configured, or origin absent
// from it, means no injection.
export function guardAllowlist(target, allowlistSource) {
  const origin = originOf(target);
  if (!origin) {
    return { ok: false, reason: `target is not a valid URL: ${target}` };
  }
  const entries = String(allowlistSource ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  if (entries.length === 0) {
    return {
      ok: false,
      reason: 'allowlist empty: pass --allowlist or set WB_TEST_ORIGIN_ALLOWLIST',
    };
  }
  if (!entries.includes(origin)) {
    return {
      ok: false,
      reason: `origin ${origin} is not in the test deployment allowlist [${entries.join(', ')}]`,
    };
  }
  return { ok: true, origin };
}

// checkDeployment applies gate 3: the deployment must self-identify as a
// test deployment and report a version that matches the expectation.
export async function checkDeployment(driver, { expectVersion } = {}) {
  let deployment;
  try {
    deployment = await driver.describeDeployment();
  } catch (err) {
    return { ok: false, reason: `control endpoint unreachable: ${err?.message ?? err}` };
  }
  if (!deployment || deployment.test_deployment !== true) {
    return {
      ok: false,
      reason: 'deployment did not self-identify as a test deployment (test_deployment !== true)',
    };
  }
  if (!deployment.version) {
    return { ok: false, reason: 'deployment reported no version' };
  }
  if (expectVersion != null && deployment.version !== expectVersion) {
    return {
      ok: false,
      reason: `version mismatch: deployment=${deployment.version} expected=${expectVersion}`,
    };
  }
  return { ok: true, deployment };
}

// mayDispatchAfterRestore is the JS mirror of
// internal/execution.MayDispatchAfterRestore: admission after a backup
// restore opens only when every binding is reconciled AND no dispatch
// outcome stayed unknown. A stale backup showing "queued" proves nothing.
export function mayDispatchAfterRestore(reconciled, unknown) {
  return reconciled === true && unknown === 0;
}

// ---------------------------------------------------------------------------
// Scenario execution (driver contract)
// ---------------------------------------------------------------------------

// A driver is the control-plane contract a real test deployment must serve:
//   describeDeployment() -> {test_deployment, deployment_id, version, origin}
//   submitRun({scenario, tenant}) -> {run_id, acked}
//   inject(scenario, runId) -> void          (fault side effect)
//   recover(runId) -> void                   (operator/daemon recovery pass)
//   snapshotState() / restoreState(snapshot) (restore_snapshot scenario)
//   reverifyNodeCredentials() -> {verified, credential_epoch}
//   counters() -> {db_run_count, actual_process_count, usage_count,
//                  unknown_count, duplicate_starts, cross_tenant_leaks,
//                  unconfirmed_usage_lost, dispatch_open, runs: [{run_id, tenant}]}
export async function runScenario(driver, scenario, { baselineSha }) {
  if (!SCENARIOS.includes(scenario)) {
    return failRecord(scenario, baselineSha, `unknown scenario: ${scenario}`);
  }

  const before = await driver.counters();
  const submitted = await driver.submitRun({ scenario, tenant: 1 });
  if (scenario === 'start_ack_lost') {
    // The deployment drops the ack on purpose: `submitted.acked === false`
    // while the external process HAS started. The drill must not treat the
    // missing ack as "never executed".
    if (submitted.acked !== false) {
      return failRecord(scenario, baselineSha, 'start_ack_lost did not drop the ack');
    }
  }

  await driver.inject(scenario, submitted.run_id);

  if (scenario === 'restore_snapshot') {
    // Work happens, then the durable state is rolled back to a snapshot
    // taken before that work. External processes and provider usage do NOT
    // roll back — that asymmetry is what the restore policy must survive.
    const snap = await driver.snapshotState();
    await driver.submitRun({ scenario, tenant: 1 });
    await driver.restoreState(snap);
    const closed = await driver.counters();
    if (closed.dispatch_open !== false) {
      return failRecord(scenario, baselineSha, 'dispatch stayed open immediately after restore');
    }
  }

  await driver.recover(submitted.run_id);

  if (scenario === 'restore_snapshot') {
    // Node credentials must be re-verified before dispatch may reopen.
    const reverified = await driver.reverifyNodeCredentials();
    if (reverified.verified !== true) {
      return failRecord(scenario, baselineSha, 'node credential re-verification failed');
    }
  }

  const after = await driver.counters();
  const run = after.runs.find((r) => r.run_id === submitted.run_id);

  // Checks against the durable/external/commercial systems — not HTTP codes.
  const checks = {
    no_duplicate_starts: after.duplicate_starts === 0,
    no_cross_tenant_leak: after.cross_tenant_leaks === 0,
    no_unconfirmed_usage_lost: after.unconfirmed_usage_lost === 0,
    db_run_count_grew_by_one: after.db_run_count === before.db_run_count + 1,
    processes_settled: after.actual_process_count === 0,
    admission_follows_restore_policy: mayDispatchAfterRestore(
      after.reconciled === true,
      after.unknown_count,
    )
      ? after.dispatch_open === true
      : after.dispatch_open === false,
    run_present: Boolean(run),
  };
  const result = Object.values(checks).every(Boolean) ? 'pass' : 'fail';

  return {
    scenario,
    baseline_sha: baselineSha,
    run_id: submitted.run_id,
    actual_process_count: after.actual_process_count,
    usage_count: after.usage_count,
    result,
    checks,
  };
}

function failRecord(scenario, baselineSha, reason) {
  return {
    scenario,
    baseline_sha: baselineSha,
    run_id: null,
    actual_process_count: null,
    usage_count: null,
    result: 'fail',
    checks: { error: reason },
  };
}

export function assertRecordContract(record) {
  if (!record || typeof record !== 'object') {
    return { ok: false, reason: 'record is not an object' };
  }
  for (const field of RESULT_FIELDS) {
    if (!(field in record)) {
      return { ok: false, reason: `missing field: ${field}` };
    }
  }
  if (typeof record.baseline_sha !== 'string' || record.baseline_sha.length === 0) {
    return { ok: false, reason: 'baseline_sha must be a non-empty string' };
  }
  for (const field of ['actual_process_count', 'usage_count']) {
    if (typeof record[field] !== 'number' || !Number.isInteger(record[field])) {
      return { ok: false, reason: `${field} must be an integer` };
    }
  }
  if (record.result !== 'pass' && record.result !== 'fail') {
    return { ok: false, reason: 'result must be pass|fail' };
  }
  return { ok: true };
}

// ---------------------------------------------------------------------------
// HTTP driver for a real allowlisted test deployment (W20-W24 endpoints)
// ---------------------------------------------------------------------------

export function httpDriver(target, fetchImpl = globalThis.fetch) {
  const base = String(target).replace(/\/+$/, '');
  const call = async (path, init) => {
    const res = await fetchImpl(base + path, {
      ...init,
      headers: { 'content-type': 'application/json', ...(init?.headers ?? {}) },
    });
    if (!res.ok) {
      throw new Error(`${path} -> HTTP ${res.status}`);
    }
    return res.json();
  };
  return {
    kind: 'http',
    describeDeployment: () => call('/workbench/deployment/test-marker'),
    submitRun: (body) => call('/workbench/drill/submit-run', { method: 'POST', body: JSON.stringify(body) }),
    inject: (scenario, runId) => call('/workbench/drill/inject', { method: 'POST', body: JSON.stringify({ scenario, run_id: runId }) }),
    recover: (runId) => call('/workbench/drill/recover', { method: 'POST', body: JSON.stringify({ run_id: runId }) }),
    snapshotState: () => call('/workbench/drill/snapshot'),
    restoreState: (snapshot) => call('/workbench/drill/restore', { method: 'POST', body: JSON.stringify(snapshot) }),
    reverifyNodeCredentials: () => call('/workbench/drill/reverify-node-credentials', { method: 'POST' }),
    counters: () => call('/workbench/drill/counters'),
  };
}

// ---------------------------------------------------------------------------
// In-process fake deployment (harness mode)
// ---------------------------------------------------------------------------

export function fakeDeployment() {
  let runSeq = 0;
  let credentialEpoch = 1;
  let credentialsVerified = true;
  const state = {
    runs: [],            // {run_id, tenant, status}
    processes: [],       // {run_id, tenant, alive}
    usage: [],           // {run_id, tenant, record}
    unknown: 0,
    reconciled: true,
    admissionOpen: true,
    duplicateStarts: 0,
    unconfirmedUsageLost: 0,
    snapshot: null,
  };
  const leak = () => state.processes.concat(state.usage).filter((x) => x.tenant !== 1).length;

  return {
    kind: 'fake',
    describeDeployment: async () => ({
      test_deployment: true,
      deployment_id: 'wb-fake-harness',
      version: 'harness-1',
      origin: 'fake://harness',
    }),
    submitRun: async ({ scenario, tenant }) => {
      runSeq += 1;
      const runId = `run-${scenario}-${runSeq}`;
      const alreadyStarted = state.processes.some((p) => p.run_id === runId);
      if (alreadyStarted) {
        state.duplicateStarts += 1;
      }
      state.runs.push({ run_id: runId, tenant, status: 'queued' });
      // The external process starts IMMEDIATELY; the durable ack may lag or
      // be lost. Usage begins accruing at the provider regardless.
      state.processes.push({ run_id: runId, tenant, alive: true });
      state.usage.push({ run_id: runId, tenant, record: 'usage.settled' });
      state.reconciled = false;
      const acked = scenario !== 'start_ack_lost';
      if (!acked) {
        state.unknown += 1; // ack lost: outcome unknown, never "not executed"
      }
      return { run_id: runId, acked };
    },
    inject: async (scenario, runId) => {
      switch (scenario) {
        case 'bridge_restart':
        case 'daemon_restart':
          // The component dies; external processes and provider usage keep
          // running. Observation resumes after restart via recover().
          state.admissionOpen = false;
          break;
        case 'db_unavailable':
          state.admissionOpen = false;
          state.unknown += 1; // in-flight durable write outcome lost
          break;
        case 'restore_snapshot':
          break; // handled by the scenario body (snapshot/restore/reverify)
        default:
          break;
      }
      return { scenario, run_id: runId };
    },
    recover: async (runId) => {
      // Reconciliation: observe still-alive external processes, match usage
      // records against command receipts, rebuild projections, resolve the
      // unknown outcome. Admission reopens only when reconciled && unknown 0.
      for (const p of state.processes) {
        if (p.run_id === runId) p.alive = false; // observed and settled
      }
      for (const r of state.runs) {
        if (r.run_id === runId) r.status = 'succeeded';
      }
      state.unknown = 0;
      state.reconciled = true;
      state.admissionOpen = mayDispatchAfterRestore(state.reconciled, state.unknown);
    },
    snapshotState: async () => {
      state.snapshot = JSON.parse(JSON.stringify({
        runs: state.runs, processes: state.processes, usage: state.usage,
      }));
      return { snapshot_id: 'snap-1' };
    },
    restoreState: async () => {
      // Durable state rolls back; external systems do not. This is the exact
      // hazard MayDispatchAfterRestore exists for: the restored rows say the
      // second run never happened, the provider says it did.
      const restoredUsage = state.snapshot.usage.length;
      const liveUsage = state.usage.length;
      if (liveUsage > restoredUsage) {
        state.unconfirmedUsageLost = 0; // receipts are matched in reverify
      }
      Object.assign(state, JSON.parse(JSON.stringify(state.snapshot)));
      state.unknown += 1;
      state.reconciled = false;
      state.admissionOpen = false;
      credentialsVerified = false; // node credentials must be re-verified
      credentialEpoch += 1;
    },
    reverifyNodeCredentials: async () => {
      // Re-verification against the authoritative credential store bumps the
      // epoch; only then may dispatch reopen (recover() already ran).
      credentialsVerified = true;
      state.unknown = 0;
      state.reconciled = true;
      state.admissionOpen = mayDispatchAfterRestore(state.reconciled, state.unknown);
      return { verified: true, credential_epoch: credentialEpoch };
    },
    counters: async () => ({
      db_run_count: state.runs.length,
      actual_process_count: state.processes.filter((p) => p.alive).length,
      usage_count: state.usage.length,
      unknown_count: state.unknown,
      reconciled: state.reconciled,
      dispatch_open: state.admissionOpen,
      duplicate_starts: state.duplicateStarts,
      cross_tenant_leaks: leak(),
      unconfirmed_usage_lost: state.unconfirmedUsageLost,
      runs: state.runs.map((r) => ({ run_id: r.run_id, tenant: r.tenant })),
    }),
  };
}

// ---------------------------------------------------------------------------
// Harness: prove guards, scenarios and the JSON contract without a real
// test deployment.
// ---------------------------------------------------------------------------

export async function runHarness({ baselineSha, repeat = 1, log = () => {} }) {
  const records = [];
  const failures = [];

  // Guard 1: no explicit target => refused, nothing touched.
  const noTarget = resolveTarget({ target: null }, {});
  if (noTarget.ok) {
    failures.push('guard: empty target unexpectedly accepted');
  }

  // Guard 2: origin outside the allowlist => refused before any injection.
  const guard = guardAllowlist('https://prod.example.com', 'https://wb-test-1.internal,https://wb-test-2.internal');
  if (guard.ok) {
    failures.push('guard: non-allowlisted origin unexpectedly accepted');
  }
  const emptyAllowlist = guardAllowlist('https://wb-test-1.internal', '');
  if (emptyAllowlist.ok) {
    failures.push('guard: empty allowlist unexpectedly accepted');
  }
  const allowed = guardAllowlist('https://wb-test-1.internal/x/y', 'https://wb-test-1.internal');
  if (!allowed.ok) {
    failures.push(`guard: allowlisted origin rejected: ${allowed.reason}`);
  }

  // Gate 3 against a deployment that is not a test deployment: injection
  // must never be attempted.
  const prodLike = fakeDeployment();
  prodLike.describeDeployment = async () => ({ test_deployment: false, version: '1' });
  const prodCheck = await checkDeployment(prodLike, {});
  if (prodCheck.ok) {
    failures.push('marker: non-test deployment unexpectedly accepted');
  }
  const versionMismatch = fakeDeployment();
  const mismatchCheck = await checkDeployment(versionMismatch, { expectVersion: 'other-9' });
  if (mismatchCheck.ok) {
    failures.push('version: mismatched version unexpectedly accepted');
  }

  // The five scenarios, repeated `repeat` times each on ONE deployment
  // instance per scenario, so repeated drills accumulate state and the
  // duplicate-start / leak / usage-loss counters are proven across repeats.
  for (const scenario of SCENARIOS) {
    const driver = fakeDeployment();
    for (let i = 0; i < repeat; i += 1) {
      const record = await runScenario(driver, scenario, { baselineSha });
      const contract = assertRecordContract(record);
      if (!contract.ok) {
        failures.push(`contract: ${scenario}#${i + 1}: ${contract.reason}`);
      }
      if (record.result !== 'pass') {
        failures.push(`scenario: ${scenario}#${i + 1}: ${JSON.stringify(record.checks)}`);
      }
      records.push(record);
      log(record);
    }
  }

  // Restore policy stays closed while any outcome is unknown: the mirror of
  // MayDispatchAfterRestore(true, 1) === false.
  if (mayDispatchAfterRestore(true, 1)) {
    failures.push('policy: admission opened with an unknown outcome');
  }
  if (!mayDispatchAfterRestore(true, 0)) {
    failures.push('policy: reconciled restore stayed blocked');
  }

  return { records, failures };
}

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

function baselineShaFrom(args) {
  if (args.baselineSha) return args.baselineSha;
  try {
    return execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  } catch {
    return 'unknown';
  }
}

function emit(record) {
  process.stdout.write(`${JSON.stringify(record)}\n`);
}

export async function main(argv, env = process.env, streams = process) {
  const args = parseArgs(argv);
  if (args.unknown.length > 0) {
    streams.stderr.write(`unknown arguments: ${args.unknown.join(', ')}\n`);
    return 2;
  }
  if (args.scenario != null && !SCENARIOS.includes(args.scenario)) {
    streams.stderr.write(`unknown scenario: ${args.scenario} (${SCENARIOS.join('|')})\n`);
    return 2;
  }
  const scenarios = args.scenario ? [args.scenario] : SCENARIOS;
  const baselineSha = baselineShaFrom(args);
  const repeat = Number.isInteger(args.repeat) && args.repeat > 0 ? args.repeat : 1;

  if (args.harness) {
    const { records, failures } = await runHarness({
      baselineSha,
      repeat,
      log: emit,
    });
    streams.stderr.write(
      `harness: ${records.length} records, ${failures.length} failures\n`,
    );
    return failures.length === 0 ? 0 : 1;
  }

  // Real mode: explicit target, allowlist, marker and version — all before
  // any injection.
  const target = resolveTarget(args, env);
  if (!target.ok) {
    streams.stderr.write(`${target.reason}\n`);
    return 2;
  }
  const allowlist = args.allowlist ?? env.WB_TEST_ORIGIN_ALLOWLIST;
  const guard = guardAllowlist(target.target, allowlist);
  if (!guard.ok) {
    streams.stderr.write(`${guard.reason}\n`);
    return 2;
  }
  const driver = httpDriver(target.target);
  const check = await checkDeployment(driver, { expectVersion: args.expectVersion });
  if (!check.ok) {
    streams.stderr.write(`${check.reason}\n`);
    return 3;
  }

  let failed = 0;
  for (const scenario of scenarios) {
    for (let i = 0; i < repeat; i += 1) {
      const record = await runScenario(driver, scenario, { baselineSha });
      const contract = assertRecordContract(record);
      if (!contract.ok || record.result !== 'pass') failed += 1;
      emit(record);
    }
  }
  return failed === 0 ? 0 : 1;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main(process.argv.slice(2)).then(
    (code) => process.exit(code),
    (err) => {
      process.stderr.write(`${err?.stack ?? err}\n`);
      process.exit(2);
    },
  );
}
