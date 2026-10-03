import { useState } from 'react';
import { Text, View } from '@tarojs/components';
import { Screen, Card, Action, Field, Notice, Badge, DataBoundary, useData, useAction, useSession } from '../components/ui.tsx';
import * as career from '../services/career.ts';
import { client, auth } from '../services/runtime.ts';
import { decodeCareerReceipt } from '../../../../packages/career-core/src/contracts.ts';
import { decodeAs } from '../services/career-intent.ts';
import { createControlledStore, intentKeyFor, recoverableWrite, retryRecoverable, readStoredIntent, type StoredIntent } from '../services/career-intent.ts';
import type { ScopeStamp } from '../core/scope.ts';
import {
  saveRule, pendingRuleWrite, reconcilePendingRule, retryPendingRule, abandonPendingRuleWrite, readStoredRuleId, readRule,
  fetchUsageEstimate, fetchReminders, createReminder, pendingReminderWrite, reconcilePendingReminder, retryPendingReminder, abandonPendingReminderWrite,
  setPushSubscription, requestReminderSubscription, REMINDER_PUSH_PRIVACY_NOTE,
  type SubscriptionRequestOutcome, type RuleWriteInput,
} from '../adapters/career-platform.ts';
import type { RuleStatus, RuleView, SetRuleReceipt, UsageEstimateView, ReminderReceipt, ReminderView, ReminderSourceKind } from '../../../../packages/api-client/src/career.ts';
import { formatTime } from '../core/format.ts';

// T30 持续规则、额度与提醒：与 Web RulePage/UsagePanel/InboxPage 同源同版本（同一批
// api-client 解码器，同一冻结合同）。规则由用户显式启停，未启用/暂停绝不触发搜索；
// 每次触发经预算准入；修改后下次运行计划由服务端按新条件重新排程（本端只呈现）。
// 额度是执行前只读预估（后端原文如实展示不重算）：超额只阻新收费动作，档案/申请/
// 时间线等历史完整可访问；重复请求按原 requestId 幂等重放，不二扣。提醒正文是服务端
// 冻结隐私模板（零公司/岗位/面试细节）；订阅消息走 wx.requestSubscribeMessage（平台
// 原生 API，已记录的原生能力例外——不在 TDesign 组件域），载荷只带模板 id；拒绝或
// 不可用一律回落站内待办并如实声明，本端绝不声称已送达。
const statusLabels: Record<RuleStatus, string> = { enabled: '已启用', paused: '已暂停', disabled: '未启用' };
const runStatusLabels: Record<string, string> = { completed: '已完成', failed: '未完成', blocked_no_quota: '额度不足（本次未执行搜索）', no_vetted_sources: '暂无已核验来源（本次未执行搜索）' };
const sourceKindLabels: Record<ReminderSourceKind, string> = { progress_event: '来源：求职进展事件', discovery: '来源：持续找岗发现' };
const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
const typedCode = (error: unknown): string | undefined => { const code = (error as { code?: unknown } | null | undefined)?.code; return typeof code === 'string' ? code : undefined; };
const formatCheckTime = (timestamp: string): string => `${timestamp.slice(0, 10)} ${timestamp.slice(11, 16)} UTC`;
const formatEstimateNumber = (value: number): string => Number.isInteger(value) ? String(value) : value.toFixed(2);
const pushOutcomeNote = (receipt: ReminderReceipt): string => {
  // push 报告是 response-only：任何投递结果都不改变站内待办事实。
  if (!receipt.push) return '';
  if (!receipt.push.attempted) return '当前已退订推送：未发送提醒，待办以站内为准。';
  return receipt.push.delivered ? '推送提醒已发送（推送只是提醒，以站内为准）。' : '推送投递失败——待办已保存，以站内为准；推送失败不改变站内事实。';
};
const pushPreferenceStore = createControlledStore();
const pushPreferenceKey = (stamp: ScopeStamp = auth.scope.capture()): string => intentKeyFor('push-subscription', stamp);
const pendingPushPreference = (stamp: ScopeStamp = auth.scope.capture()): StoredIntent<{ value: 'subscribed' }> | null => readStoredIntent(pushPreferenceStore, pushPreferenceKey(stamp));
const pushAuthorizationKey = (stamp: ScopeStamp = auth.scope.capture()): string => intentKeyFor('push-authorization-pending', stamp);
const pushInvalidReceiptKey = (stamp: ScopeStamp = auth.scope.capture()): string => intentKeyFor('push-receipt-invalid', stamp);
function pushAuthorizationPending(stamp: ScopeStamp = auth.scope.capture()): boolean { return pushPreferenceStore.read(pushAuthorizationKey(stamp)) === true; }
function pushReceiptInvalid(stamp: ScopeStamp = auth.scope.capture()): boolean { return pushPreferenceStore.read(pushInvalidReceiptKey(stamp)) === true; }
export function hasInvalidPushReceipt(stamp: ScopeStamp): boolean { return pushReceiptInvalid(stamp); }
export function clearPushReceiptInvalidAfterProfileRefresh(stamp: ScopeStamp): boolean {
  if (!auth.scope.isCurrent(stamp)) return false;
  pushPreferenceStore.remove(pushInvalidReceiptKey(stamp));
  pushPreferenceStore.remove(pushAuthorizationKey(stamp));
  return true;
}
export function decodePushPreferenceReceipt(value: unknown): ReturnType<typeof decodeCareerReceipt> { return decodeAs(decodeCareerReceipt, value); }
function sendPushPreference(id: string, expected: number): Promise<ReturnType<typeof decodeCareerReceipt>> {
  return client.request({ method: 'POST', path: '/api/v1/career/act', body: { action: 'confirm', key: 'notifications.push', value: 'subscribed', source: { kind: 'user', label: '微信小程序' }, requestId: id, expectedRevision: expected } }).then(decodePushPreferenceReceipt);
}
export async function persistPushPreference(expectedRevision: number, stamp: ScopeStamp = auth.scope.capture()): Promise<ReturnType<typeof decodeCareerReceipt>> {
  if (!auth.scope.isCurrent(stamp)) throw Object.assign(new Error('SCOPE_CHANGED'), { code: 'SCOPE_CHANGED' });
  if (pushReceiptInvalid(stamp)) throw Object.assign(new Error('请先读取服务器订阅偏好事实'), { code: 'invalid_receipt' });
  try {
    return await recoverableWrite(pushPreferenceStore, { kind: 'push-subscription', describe: '推送订阅偏好', input: { value: 'subscribed' as const }, expected: expectedRevision, send: sendPushPreference });
  } catch (error) {
    if (typedCode(error) === 'contract_violation' && auth.scope.isCurrent(stamp)) pushPreferenceStore.write(pushInvalidReceiptKey(stamp), true);
    throw error;
  }
}
export async function retryPushPreference(stamp: ScopeStamp = auth.scope.capture()): Promise<ReturnType<typeof decodeCareerReceipt>> {
  if (!auth.scope.isCurrent(stamp)) throw Object.assign(new Error('SCOPE_CHANGED'), { code: 'SCOPE_CHANGED' });
  if (pushReceiptInvalid(stamp)) throw Object.assign(new Error('请先读取服务器订阅偏好事实'), { code: 'invalid_receipt' });
  return retryRecoverable(pushPreferenceStore, 'push-subscription', '推送订阅偏好', sendPushPreference);
}
async function reconcilePushPreference(): Promise<ReturnType<typeof decodeCareerReceipt>> {
  const stamp = auth.scope.capture();
  const pending = pendingPushPreference(stamp);
  if (!pending) throw new Error('没有待对账的推送订阅偏好');
  const receipt = decodeAs(decodeCareerReceipt, await client.request({ method: 'GET', path: `/api/v1/career/receipt?requestId=${encodeURIComponent(pending.requestId)}` }));
  if (!auth.scope.isCurrent(stamp)) throw Object.assign(new Error('scope changed during push preference reconciliation'), { code: 'SCOPE_CHANGED' });
  pushPreferenceStore.remove(pushPreferenceKey(stamp));
  return receipt;
}

export interface PushSubscribeFlowDeps {
  stamp: ScopeStamp;
  isCurrent(stamp: ScopeStamp): boolean;
  hasPendingWrite(stamp: ScopeStamp): boolean;
  hasInvalidReceipt(stamp: ScopeStamp): boolean;
  marker(stamp: ScopeStamp): boolean;
  setMarker(stamp: ScopeStamp): void;
  clearMarker(stamp: ScopeStamp): void;
  requestAuthorization(): Promise<SubscriptionRequestOutcome>;
  revision(): number | undefined;
  persist(revision: number, stamp: ScopeStamp): Promise<unknown>;
}
export async function runPushSubscribeFlow(deps: PushSubscribeFlowDeps): Promise<{ status: 'saved' | 'missing_revision' | 'rejected' | 'unavailable' | 'scope_changed' | 'invalid_receipt'; outcome?: SubscriptionRequestOutcome }> {
  const { stamp } = deps;
  if (deps.hasPendingWrite(stamp)) return { status: 'scope_changed' };
  if (deps.hasInvalidReceipt(stamp)) return { status: 'invalid_receipt' };
  const alreadyAuthorized = deps.marker(stamp);
  const outcome = alreadyAuthorized ? undefined : await deps.requestAuthorization();
  if (!deps.isCurrent(stamp)) return { status: 'scope_changed' };
  if (outcome && outcome.status !== 'accepted') return { status: outcome.status, outcome };
  if (outcome?.status === 'accepted') deps.setMarker(stamp);
  const revision = deps.revision();
  if (!Number.isSafeInteger(revision) || revision === undefined || revision < 0) return { status: 'missing_revision', outcome };
  if (!deps.isCurrent(stamp)) return { status: 'scope_changed' };
  await deps.persist(revision, stamp);
  if (!deps.isCurrent(stamp)) return { status: 'scope_changed' };
  deps.clearMarker(stamp);
  return { status: 'saved', outcome };
}
export function clearPushAuthorizationAfterOptOut(stamp: ScopeStamp, isCurrent: (stamp: ScopeStamp) => boolean, clear: (stamp: ScopeStamp) => void): boolean {
  if (!isCurrent(stamp)) return false;
  clear(stamp);
  return true;
}
export async function runPushOptOut(stamp: ScopeStamp, isCurrent: (stamp: ScopeStamp) => boolean, clearAuthorization: (stamp: ScopeStamp) => void, persist: () => Promise<unknown>): Promise<boolean> {
  if (!isCurrent(stamp)) return false;
  clearAuthorization(stamp);
  try { await persist(); } catch (error) {
    if (typedCode(error) === 'contract_violation' && isCurrent(stamp)) pushPreferenceStore.write(pushInvalidReceiptKey(stamp), true);
    throw error;
  }
  return isCurrent(stamp);
}

export default function RulesUsageRemindersPage() {
  const session = useSession();
  const desk = useData(`career:${session.userId}:${session.tenantId}`, () => career.loadCareer());
  const usageBusy = useAction(); const loadRuleBusy = useAction(); const saveBusy = useAction();
  const recRuleBusy = useAction(); const retryRuleBusy = useAction();
  const inboxBusy = useAction(); const remindBusy = useAction();
  const recRemindBusy = useAction(); const retryRemindBusy = useAction();
  const subscribeBusy = useAction(); const optOutBusy = useAction();
  const recPushBusy = useAction(); const retryPushBusy = useAction();

  // —— 额度预估（执行前只读）——
  const [estimate, setEstimate] = useState<UsageEstimateView>();
  const [usageErrCode, setUsageErrCode] = useState<string>();
  // —— 规则表单与当前版本 ——
  const [query, setQuery] = useState('');
  const [intervalText, setIntervalText] = useState('1440');
  const [status, setStatus] = useState<RuleStatus>('disabled');
  const [ruleView, setRuleView] = useState<RuleView>();
  const [receipt, setReceipt] = useState<SetRuleReceipt>();
  const [ruleErrCode, setRuleErrCode] = useState<string>();
  const [ruleNotice, setRuleNotice] = useState('');
  // —— 站内待办与订阅消息 ——
  const [todos, setTodos] = useState<ReminderView[]>();
  const [sourceKind, setSourceKind] = useState<ReminderSourceKind>('progress_event');
  const [sourceId, setSourceId] = useState('');
  const [reminderNotice, setReminderNotice] = useState('');
  const [subscription, setSubscription] = useState<SubscriptionRequestOutcome>();
  const [pushNotice, setPushNotice] = useState('');

  const pendingRule = pendingRuleWrite();
  const pendingReminder = pendingReminderWrite();
  const pageScope = auth.scope.capture();
  const pendingPush = pendingPushPreference(pageScope);
  const revision = desk.data?.revision;
  const intervalNumber = Number(intervalText);
  const intervalValid = Number.isSafeInteger(intervalNumber) && intervalNumber >= 1 && intervalNumber <= 43200;
  // T21 口径：启用是收费路径（每次触发=一次 search_once），必须先有可用预估（fail-closed，
  // 不会先执行后补报）；暂停/停用保持开放（永远不会触发收费运行）。
  const enableRequiresEstimate = status === 'enabled' && estimate === undefined;
  const existingRuleId = receipt?.ruleId ?? ruleView?.ruleId;
  const live = receipt ?? ruleView;

  const loadEstimate = async (): Promise<void> => {
    setUsageErrCode(undefined);
    try { setEstimate(await fetchUsageEstimate()); } catch (error) { setUsageErrCode(typedCode(error) ?? 'error'); throw error; }
  };
  const loadRule = async (): Promise<void> => {
    setRuleErrCode(undefined);
    const stored = readStoredRuleId();
    if (!stored) { setRuleNotice('本机还没有保存过的规则引用。保存第一条规则后，这里会按同一规则编号读回同一版本。'); setRuleView(undefined); return; }
    // 读取失败也要分型（OCR med-40）：not_found/forbidden 的定制提示依赖 ruleErrCode
    // 先落 state，再原样 rethrow 交给 useAction 呈现。
    try {
      const view = await readRule(stored);
      setRuleView(view); setReceipt(undefined);
      setQuery(view.query); setIntervalText(String(view.intervalMinutes)); setStatus(view.status);
      setRuleNotice(`已读取规则 ${view.ruleId.slice(0, 10)}…（修订 ${view.revision}，与 Web 同一版本）。`);
    } catch (error) { setRuleErrCode(typedCode(error)); throw error; }
  };
  const acceptRuleReceipt = (next: SetRuleReceipt): void => {
    setReceipt(next); setRuleErrCode(undefined); setRuleNotice('');
    void loadRuleBusy.run(loadRule);
    // 已保存规则（尤其启用中）改变下一次收费运行的口径——重读实时预估而不是展示陈旧余额。
    if (estimate !== undefined) void usageBusy.run(loadEstimate);
  };

  return <Screen title='持续找岗与提醒'>
    <Text className='wk-display'>规则你开你停，{'\n'}额度先看后用，提醒只说“有更新”。</Text>
    <Notice tone='info'>与 Web 端同一后端合同：持续找岗规则默认不开启，未开启或已暂停不会触发任何搜索；每次触发都经预算准入。{REMINDER_PUSH_PRIVACY_NOTE}</Notice>

    {/* 恢复态置顶（T24/T26/T32 教训；T32-M1 口径：未知/对账中主按钮禁用+重新对账入口） */}
    {pendingRule && <>
      <Notice tone='warning'>有一次结果未知的规则保存（{pendingRule.requestId.slice(0, 10)}…）。请先用原请求对账，不会写入第二条规则；对账无记录后可用原请求编号安全重发。</Notice>
      <Action secondary loading={recRuleBusy.busy} onClick={() => void recRuleBusy.run(async () => { acceptRuleReceipt(await reconcilePendingRule()); })}>用原请求对账规则保存</Action>
      {recRuleBusy.error && <Notice tone='danger'>{recRuleBusy.error} 对账被拒时说明该请求不存在或不属于当前空间；可再用原编号重试（幂等重放）。</Notice>}
      <Action secondary loading={retryRuleBusy.busy} onClick={() => void retryRuleBusy.run(async () => {
        try { acceptRuleReceipt(await retryPendingRule()); } catch (error) { setRuleErrCode(typedCode(error)); throw error; }
      })}>用原请求编号重试规则保存</Action>
      {retryRuleBusy.error && <Notice tone='danger'>{retryRuleBusy.error}{ruleErrCode === 'revision_conflict_abandoned' ? ' 本次恢复已结束（保存按钮恢复可用）。' : ' 重试沿用原请求编号与原档案修订，服务端幂等不会写入第二条规则。'}</Notice>}
      {/* OCR high-12 出口：重试收到其它确定失败时恢复链可能无解，提供显式放弃——只清
          本端 intent，不动服务端事实；若原保存实际已落地，按规则编号读回即可找回。 */}
      <Action secondary onClick={() => { abandonPendingRuleWrite(); setRuleErrCode(undefined); setRuleNotice('已放弃本次恢复：保存按钮恢复可用。若原保存实际已生效，规则以服务端记录为准——可点「读回已保存的规则」找回，不会写出第二条规则。'); }}>放弃本次恢复（保存按钮恢复可用）</Action>
    </>}
    {pendingReminder && <>
      <Notice tone='warning'>有一次结果未知的待办登记（{pendingReminder.requestId.slice(0, 10)}…）。请先用原请求对账。</Notice>
      <Action secondary loading={recRemindBusy.busy} onClick={() => void recRemindBusy.run(async () => { await reconcilePendingReminder(); setReminderNotice('已对账到待办回执。'); void inboxBusy.run(loadInbox); })}>用原请求对账待办登记</Action>
      {recRemindBusy.error && <Notice tone='danger'>{recRemindBusy.error}</Notice>}
      <Action secondary loading={retryRemindBusy.busy} onClick={() => void retryRemindBusy.run(async () => { await retryPendingReminder(); setReminderNotice('待办登记已用原请求编号恢复完成。'); void inboxBusy.run(loadInbox); })}>用原请求编号重试待办登记</Action>
      {retryRemindBusy.error && <Notice tone='danger'>{retryRemindBusy.error}</Notice>}
      <Action secondary onClick={() => { abandonPendingReminderWrite(); setReminderNotice('已放弃本次待办恢复：登记入口恢复可用。若原登记实际已生效，待办以服务端记录为准——刷新站内待办即可见。'); }}>放弃本次待办恢复</Action>
    </>}

    {/* —— 额度预估：执行前只读呈现（后端原文，不重算）—— */}
    <Card>
      <Text className='wk-h3'>额度预估（执行前）</Text>
      <Action secondary loading={usageBusy.busy} onClick={() => void usageBusy.run(loadEstimate)}>{estimate ? '重新获取预估' : '获取额度预估'}</Action>
      {usageBusy.error && <Notice tone='danger'>{usageBusy.error} 预估暂不可用时不会发起收费找岗（不会先执行后补报）；期间既有档案、申请与搜索记录仍可完整读取，可稍后重试。</Notice>}
      {estimate && <>
        <Text className='wk-row-title'>下一次找岗将消耗 {estimate.costUnits} 个额度单位（一次性找岗；持续规则的每次触发按同一口径计）。</Text>
        {!estimate.wouldAdmit && <Notice tone='warning'>本期额度已耗尽：新的收费找岗已被阻止；既有档案、申请、评估与搜索记录仍可完整读取。额度在新的计费窗口自动恢复。</Notice>}
        <Text className='wk-muted wk-small'>本期剩余 {estimate.remainingUnits} / {estimate.limitUnits}（已预占 {estimate.reservedUnits} · 已结算 {estimate.settledUnits}） · 计费窗口 {estimate.periodStart.slice(0, 10)} 至 {estimate.periodEnd.slice(0, 10)} UTC</Text>
        <Text className='wk-muted wk-small'>触发条件（后端原文）：</Text>
        {estimate.conditions.map((condition, index) => <Text key={index} className='wk-muted wk-small'>· {condition}</Text>)}
        <Text className='wk-muted wk-small'>以上为后端只读预估，前端如实展示、不重算；余额与判定以服务端额度台账为准。</Text>
      </>}
    </Card>

    {/* —— 规则：与 Web 同版本读写 + 下次运行计划 —— */}
    <Card>
      <Text className='wk-h3'>持续找岗规则</Text>
      <DataBoundary state={desk}>{loaded => <Text className='wk-muted wk-small'>当前档案修订 {loaded!.revision}（规则写入按此修订提交，与 Web 同一 CAS 域）</Text>}</DataBoundary>
      <Action secondary loading={loadRuleBusy.busy} onClick={() => void loadRuleBusy.run(loadRule)}>{readStoredRuleId() ? '读回已保存的规则' : '读取规则'}</Action>
      {loadRuleBusy.error && <Notice tone='danger'>{loadRuleBusy.error}{ruleErrCode === 'not_found' ? ' 这条规则在服务端已不可见，可重新创建一条规则。' : ruleErrCode === 'forbidden' ? ' 当前空间不可访问此规则。' : ' 可稍后重试。'}</Notice>}
      {ruleNotice && <Notice tone='info'>{ruleNotice}</Notice>}
      <Field label='找岗条件（一句话描述要持续找的岗位）' value={query} onChange={setQuery} placeholder='例如：上海 前端开发 实习' multiline />
      <Field label='触发间隔（分钟，1–43200，即最长 30 天）' value={intervalText} onChange={setIntervalText} placeholder='1440' type='number' />
      <Text className='wk-muted wk-small'>规则状态（默认不开启）</Text>
      {(Object.keys(statusLabels) as RuleStatus[]).map(option => <Text key={option} className='wk-small' onClick={() => setStatus(option)}>{status === option ? '● ' : '○ '}{option === 'enabled' ? '启用（按间隔自动触发）' : option === 'paused' ? '暂停（取消下一次触发，恢复后顺延）' : '停用（不运行）'}</Text>)}
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='保存规则' customStyle={tdesignButtonStyle} loading={saveBusy.busy} disabled={!query.trim() || !intervalValid || revision === undefined || enableRequiresEstimate || pendingRule !== null} onTap={() => void saveBusy.run(async () => {
          const input: RuleWriteInput = { ...(existingRuleId ? { ruleId: existingRuleId } : {}), query: query.trim(), intervalMinutes: intervalNumber, status, expectedRevision: revision! };
          try { acceptRuleReceipt(await saveRule(input)); } catch (error) { setRuleErrCode(typedCode(error)); throw error; }
        })}>保存规则</t-button>
      </View></View>
      {!intervalValid && <Notice tone='danger'>触发间隔需为 1–43200 的整数分钟。</Notice>}
      {enableRequiresEstimate && <Notice tone='warning'>启用需要先取得可用的额度预估；预估恢复前不能启用（不会先执行后补报）。暂停与停用不受影响。</Notice>}
      {saveBusy.error && <Notice tone='danger'>{saveBusy.error}{ruleErrCode === 'revision_conflict' ? ' 档案已更新：请重新读取修订后再保存（新保存会使用新的请求编号）。' : ruleErrCode === 'idempotency_conflict' ? ' 本次请求与已保存的规则内容不一致，已放弃；请重新保存。' : ruleErrCode === 'outcome_unknown' ? ' 保存结果未知：请用页首「用原请求对账规则保存」恢复（幂等可重放），本页不会自动重发。' : ''}</Notice>}
      {ruleErrCode === 'revision_conflict' && <Action secondary onClick={() => desk.reload()}>重新读取档案修订</Action>}
      {live && <>
        <View className='wk-between'>
          <Badge tone={live.status === 'enabled' ? 'success' : live.status === 'paused' ? 'warning' : 'neutral'}>{statusLabels[live.status]}</Badge>
          <Text className='wk-muted wk-small'>规则 {live.ruleId.slice(0, 10)}… · 修订 {live.revision}</Text>
        </View>
        <Text className='wk-row-title'>「{live.query}」 · 每 {live.intervalMinutes} 分钟触发一次</Text>
        <Text className='wk-muted wk-small'>下次运行计划</Text>
        {live.status === 'enabled' && live.nextDueAt
          ? <Text className='wk-row-title'>下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</Text>
          : live.status === 'paused'
            ? <Text className='wk-muted wk-small'>已暂停：下一次触发已取消，当前没有排程。恢复启用后按恢复时刻重新排程（顺延，不追补暂停期间的周期）。</Text>
            : <Text className='wk-muted wk-small'>规则未启用：不会运行，也不会在后台执行任何搜索。启用后才会排出下次运行计划。</Text>}
        <Text className='wk-muted wk-small'>预计消耗（启用前确认，后端确定性估算）：每天预计触发 {formatEstimateNumber(live.estimate.triggersPerDay)} 次 · 每次检索 {live.estimate.sourcesPerTrigger} 个已核验来源 · 每天预计消耗 {formatEstimateNumber(live.estimate.estimatedSearchesPerDay)} 次搜索。</Text>
        <Text className='wk-muted wk-small'>估算口径（后端原文）：{live.estimate.basis}</Text>
      </>}
    </Card>

    {ruleView && <Card>
      <Text className='wk-h3'>执行历史与发现待办</Text>
      {ruleView.runs.length ? ruleView.runs.map(run => <View key={`${run.period}`} className='wk-listrow'>
        <View className='wk-grow'>
          <Text className='wk-row-title'>第 {run.period} 次 · {runStatusLabels[run.status] ?? run.status}</Text>
          <Text className='wk-muted wk-small'>{formatTime(run.triggeredAt)}{run.searchId ? ` · 搜索 ${run.searchId.slice(0, 10)}…` : ''}{run.failureCode ? ` · 失败代码 ${run.failureCode}` : ''}{run.note ? ` · ${run.note}` : ''}</Text>
        </View>
      </View>) : <Text className='wk-muted wk-small'>还没有执行记录。规则启用并到达触发时间后，每次触发（包括被预算或来源拦下的触发）都会留下一条可见记录。</Text>}
      {ruleView.todos.length ? ruleView.todos.map(todo => <View key={todo.todoId} className='wk-listrow'>
        <View className='wk-grow'>
          <Text className='wk-row-title'>发现待办 · 发现于 {formatTime(todo.createdAt)}</Text>
          <Text className='wk-muted wk-small'>{todo.sourceId ? `来源 ${todo.sourceId.slice(0, 10)}…` : '来源未登记'} · {todo.link}</Text>
        </View>
      </View>) : <Text className='wk-muted wk-small'>暂无发现待办。同一岗位链接只生成一条待办（服务端按链接去重）。</Text>}
    </Card>}

    {/* —— 订阅消息（wx.requestSubscribeMessage 原生例外）+ 站内待办收件箱 —— */}
    <Card>
      <Text className='wk-h3'>提醒与站内待办</Text>
      <Notice tone='info'>{REMINDER_PUSH_PRIVACY_NOTE}拒绝订阅或环境不支持时，站内待办始终可用；这里绝不显示“已送达”。</Notice>
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='订阅提醒' customStyle={tdesignButtonStyle} loading={subscribeBusy.busy} onTap={() => void subscribeBusy.run(async () => {
          const stamp = auth.scope.capture();
          if (pendingPushPreference(stamp)) {
            setPushNotice('已有一次订阅偏好写入结果未知；请先对账或安全重发原请求。不会再次请求微信授权。');
            return;
          }
          const result = await runPushSubscribeFlow({
            stamp,
            isCurrent: captured => auth.scope.isCurrent(captured),
            hasPendingWrite: captured => !!pendingPushPreference(captured),
            hasInvalidReceipt: captured => pushReceiptInvalid(captured),
            marker: captured => pushAuthorizationPending(captured),
            setMarker: captured => pushPreferenceStore.write(pushAuthorizationKey(captured), true),
            clearMarker: captured => pushPreferenceStore.remove(pushAuthorizationKey(captured)),
            requestAuthorization: requestReminderSubscription,
            revision: () => revision,
            persist: (expected, captured) => persistPushPreference(expected, captured),
          });
          if (!auth.scope.isCurrent(stamp) || result.status === 'scope_changed') return;
          if (result.outcome) setSubscription(result.outcome);
          if (result.status === 'invalid_receipt') { setPushNotice('服务器返回了无效的订阅偏好回执。为避免重复写入，请先读取档案确认 notifications.push 当前事实。'); return; }
          if (result.status === 'missing_revision') { setPushNotice('微信授权已接受，但当前没有可用的档案修订，服务器订阅偏好未确认。请刷新档案后再同步偏好。'); return; }
          if (result.status === 'saved') setPushNotice('微信授权已接受，服务器订阅偏好也已保存；这不代表提醒已送达。');
          void inboxBusy.run(loadInbox);
        })}>订阅提醒（微信订阅消息）</t-button>
      </View></View>
      {subscription?.status === 'accepted' && <Notice tone='info'>{pushNotice || '已授权订阅消息：本次授权只代表允许发送；是否真的送达由服务端后续推送决定，站内待办始终是完整事实。'}{REMINDER_PUSH_PRIVACY_NOTE}</Notice>}
      {subscription?.status === 'rejected' && <Notice tone='warning'>你{subscription.reason === 'main_switch_off' ? '已在微信设置中关闭订阅消息' : '拒绝了本次订阅'}：不会发送订阅消息，也绝不显示“已送达”；站内待办仍完整可读（点下方“查看站内待办”）。</Notice>}
      {subscription?.status === 'unavailable' && <Notice tone='warning'>订阅消息当前不可用（{subscription.reason === 'no_templates' ? '本构建未配置订阅消息模板 id——模板需在微信公众平台与本 appid 绑定后申请' : subscription.reason === 'api_unavailable' ? '当前环境没有 wx.requestSubscribeMessage（模拟器或基础库不支持）' : `原生调用失败${subscription.errMsg ? `：${subscription.errMsg}` : ''}`}）。如实告知未订阅：站内待办为准，不伪造已送达。</Notice>}
      {subscription && <Text className='wk-muted wk-small'>订阅请求只携带模板 id，不含任何找岗条件或岗位内容。</Text>}
      {pendingPush && <>
        <Notice tone='warning'>微信授权已接受，但服务器订阅偏好写入结果未知（原请求 {pendingPush.requestId.slice(0, 10)}…）。恢复操作不会再次请求微信授权。</Notice>
        <Action secondary loading={recPushBusy.busy} onClick={() => void recPushBusy.run(async () => { const stamp = auth.scope.capture(); await reconcilePushPreference(); if (!auth.scope.isCurrent(stamp)) return; pushPreferenceStore.remove(pushAuthorizationKey(stamp)); setPushNotice('服务器订阅偏好已对账确认；授权不代表提醒已送达。'); })}>对账原订阅偏好请求</Action>
        <Action secondary disabled={pushReceiptInvalid(pageScope)} loading={retryPushBusy.busy} onClick={() => void retryPushBusy.run(async () => { const stamp = auth.scope.capture(); await retryPushPreference(stamp); if (!auth.scope.isCurrent(stamp)) return; pushPreferenceStore.remove(pushAuthorizationKey(stamp)); setPushNotice('服务器订阅偏好已用原请求安全重发保存；授权不代表提醒已送达。'); })}>安全重发原订阅偏好</Action>
        {recPushBusy.error && <Notice tone='danger'>{recPushBusy.error} 对账失败时 intent 保留；稍后可继续恢复。</Notice>}
        {retryPushBusy.error && <Notice tone='danger'>{retryPushBusy.error} 原请求和修订已保留；不会再次请求微信授权。</Notice>}
      </>}
      {pushReceiptInvalid(pageScope) && <>
        <Notice tone='warning'>服务器成功响应的回执格式无效。当前已阻止再次写入，请先读取档案事实确认订阅偏好。</Notice>
        <Action secondary loading={recPushBusy.busy} onClick={() => void recPushBusy.run(async () => {
          const stamp = auth.scope.capture();
          const refreshed = await career.refreshCareer();
          if (!auth.scope.isCurrent(stamp) || !refreshed) return;
          const fact = refreshed.facts.find(item => item.key === 'notifications.push');
          clearPushReceiptInvalidAfterProfileRefresh(stamp);
          setPushNotice(fact ? `已读取服务器订阅偏好：${fact.value}（来自服务端档案事实）；如需更改，请重新授权后操作。` : '已读取档案，尚无 notifications.push 事实；可以重新请求微信授权后同步偏好。');
        })}>读取服务器订阅偏好事实</Action>
      </>}
      {!pendingPush && pushAuthorizationPending() && <>
        <Notice tone='warning'>微信授权已接受，但缺少可用档案修订，尚未写入服务器偏好。档案修订就绪后点「订阅提醒」同步；不会再次请求微信授权。</Notice>
        <Action secondary disabled={revision === undefined || pushReceiptInvalid(pageScope)} loading={subscribeBusy.busy} onClick={() => void subscribeBusy.run(async () => {
          if (revision === undefined) return;
          const stamp = auth.scope.capture();
          await persistPushPreference(revision, stamp);
          if (!auth.scope.isCurrent(stamp)) return;
          pushPreferenceStore.remove(pushAuthorizationKey(stamp));
          setPushNotice('服务器订阅偏好已保存；微信授权不代表提醒已送达。');
        })}>同步已授权的服务器订阅偏好</Action>
      </>}
      <Action secondary loading={optOutBusy.busy} onClick={() => void optOutBusy.run(async () => {
        if (revision === undefined) throw new Error('请先读取档案修订');
        const stamp = auth.scope.capture();
        if (!auth.scope.isCurrent(stamp)) return;
        const current = await runPushOptOut(stamp, captured => auth.scope.isCurrent(captured), captured => { pushPreferenceStore.remove(pushAuthorizationKey(captured)); }, () => setPushSubscription('unsubscribed', revision));
        if (!current) return;
        pushPreferenceStore.remove(pushInvalidReceiptKey(stamp));
        setPushNotice('推送提醒已退订：不再发送推送，已存在的站内待办仍可读取（随时可重新订阅）。');
        void inboxBusy.run(loadInbox);
      })}>退订推送提醒（站内待办不受影响）</Action>
      {optOutBusy.error && <Notice tone='danger'>{optOutBusy.error} 可重新读取档案后重试。</Notice>}
      {pushNotice && <Notice tone='info'>{pushNotice}</Notice>}
      <Action secondary loading={inboxBusy.busy} onClick={() => void inboxBusy.run(loadInbox)}>{todos ? '刷新站内待办' : '查看站内待办'}</Action>
      {inboxBusy.error && <Notice tone='danger'>{inboxBusy.error} 可稍后重试。</Notice>}
      {(todos ?? []).map(item => <View key={item.reminderId} className='wk-listrow'>
        <View className='wk-grow'>
          <Text className='wk-row-title'>{item.notice}</Text>
          <Text className='wk-muted wk-small'>{sourceKindLabels[item.sourceKind]} · {formatTime(item.createdAt)}{item.applicationId ? ` · 申请 ${item.applicationId.slice(0, 10)}…` : ''}</Text>
        </View>
      </View>)}
      {todos && todos.length === 0 && <Text className='wk-muted wk-small'>暂无站内待办。</Text>}
      <Text className='wk-muted wk-small'>为来源事件登记待办（同一来源事件只保留一条，重复登记返回去重回执）</Text>
      <Text className='wk-muted wk-small'>来源类型</Text>
      {(Object.keys(sourceKindLabels) as ReminderSourceKind[]).map(option => <Text key={option} className='wk-small' onClick={() => setSourceKind(option)}>{sourceKind === option ? '● ' : '○ '}{sourceKindLabels[option]}</Text>)}
      <Field label='来源事件编号' value={sourceId} onChange={setSourceId} placeholder='例如进展事件编号' />
      <Action secondary loading={remindBusy.busy} disabled={revision === undefined || pendingReminder !== null} onClick={() => void remindBusy.run(async () => {
        const next = await createReminder({ sourceKind, sourceId, expectedRevision: revision! });
        setReminderNotice([next.deduplicated ? '该来源已有待办：同一来源事件只保留一条，未新增第二条。' : '待办已登记。', pushOutcomeNote(next)].filter(Boolean).join(' '));
        void inboxBusy.run(loadInbox);
      })}>登记站内待办</Action>
      {remindBusy.error && <Notice tone='danger'>{remindBusy.error}</Notice>}
      {reminderNotice && <Notice tone='info'>{reminderNotice}</Notice>}
    </Card>

    <Notice tone='info'>额度耗尽时仍可读取既有档案和申请：本页档案修订、「申请与材料」「申请进展与准备」各页的历史读取不受额度影响，只有新的收费找岗被阻止。</Notice>
  </Screen>;

  async function loadInbox(): Promise<void> {
    setTodos((await fetchReminders()).reminders);
  }
}
