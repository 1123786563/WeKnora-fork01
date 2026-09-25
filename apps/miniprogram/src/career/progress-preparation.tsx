import { useState } from 'react';
import Taro from '@tarojs/taro';
import { Text, View } from '@tarojs/components';
import { Screen, Card, Action, Field, Notice, Badge, DataBoundary, useData, useAction, useSession } from '../components/ui.tsx';
import * as career from '../services/career.ts';
import { savePreparationDraft, readPreparationDraft, clearPreparationDraft, type PreparationDraftRecord } from '../adapters/career-platform.ts';
import type { ProgressView, ProgressEventView, ProgressEventType, ProgressReceipt, PreparationReceipt, PreparationFocus, MaterialBody } from '../../../../packages/api-client/src/career.ts';
import { formatTime } from '../core/format.ts';

// T28 申请进展时间线与按需准备：与 Web ProgressPage/PreparationPage 同源同版本（同一批
// api-client 解码器，同一冻结合同）。时间线 append-only：事件按服务端权威顺序（seq）
// 呈现，跨端更正后原文+更正并见（旧事件可追溯）；纠错=追加引用事件。准备锚定实际投递
// 版本（锚定可见），未确认投递→typed 提示态（409 preparation_version_unknown，绝不静默
// 改用最新版）；草稿物化 materials 域可修订（claims 原样保留）。断网只保留本地草稿
// （scope 隔离、登出即清），不静默改申请状态，重联网不自动提交——同步永远是显式动作。
const stageLabels: Record<string, string> = { preparing: '准备中', pending_submission: '待投递', submitted: '已投递', assessment: '测评或笔试', interview: '面试', offer: 'Offer', closed: '已结束' };
const eventTypeLabels: Record<string, string> = { pending_submission: '待投递', submitted: '已投递', assessment: '测评或笔试', interview: '面试', offer: 'Offer', resubmitted: '重新投递', rejected: '未通过', withdrawn: '已撤回', retracted: '招聘方撤回' };
const EVENT_TYPES = Object.keys(eventTypeLabels) as ProgressEventType[];
const sourceKindLabels: Record<string, string> = { manual: '用户录入', user: '用户录入', system_import: '系统导入' };
const focusLabels: Record<string, string> = { interview_prep: '面试准备', cover_letter: '求职信' };
const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
const digestHead = (digest: string): string => digest.slice(0, 12);
const typedCode = (error: unknown): string | undefined => { const code = (error as { code?: unknown } | null | undefined)?.code; return typeof code === 'string' ? code : undefined; };
/** 断网失败判据：transport 的 request:fail / 超时类 errMsg——用于本地草稿提示文案；
 * 判定失败与否与 intent 持久化始终由 service 层的 ambiguous 判据决定，本函数不参与。 */
const looksOffline = (error: unknown): boolean => /request:fail|network|timeout|请求超时|网络/i.test(`${(error as { cause?: { errMsg?: string } })?.cause?.errMsg ?? ''} ${(error as Error)?.message ?? ''}`);

export default function ProgressPreparationPage() {
  const session = useSession();
  const desk = useData(`career:${session.userId}:${session.tenantId}`, () => career.loadCareer());
  const loadBusy = useAction(); const appendBusy = useAction(); const correctBusy = useAction();
  const genBusy = useAction(); const listBusy = useAction(); const reviseBusy = useAction(); const readBackBusy = useAction();
  const recProgBusy = useAction(); const retryProgBusy = useAction();
  const recPrepBusy = useAction(); const retryPrepBusy = useAction(); const retryMatBusy = useAction();

  const params = Taro.getCurrentInstance().router?.params as Record<string, string | undefined> | undefined;
  const [applicationId, setApplicationId] = useState(params?.applicationId ?? '');
  const [view, setView] = useState<ProgressView>();
  const [viewErrCode, setViewErrCode] = useState<string>();
  // —— 录入/纠错表单 ——
  const [eventType, setEventType] = useState<ProgressEventType>('interview');
  const [note, setNote] = useState('');
  const [occurredAt, setOccurredAt] = useState('');
  const [appendErrCode, setAppendErrCode] = useState<string>();
  const [correcting, setCorrecting] = useState<ProgressEventView>();
  const [correctErrCode, setCorrectErrCode] = useState<string>();
  const [progressNotice, setProgressNotice] = useState('');
  // —— 按需准备 ——
  const [focus, setFocus] = useState<PreparationFocus>('interview_prep');
  const [genErrCode, setGenErrCode] = useState<string>();
  const [genNotice, setGenNotice] = useState('');
  const [preparations, setPreparations] = useState<PreparationReceipt[]>();
  const [editing, setEditing] = useState<PreparationReceipt>();
  // F4 同款：多节全量编辑模型——claims 不在本端编辑，但保存时原样回传，绝不丢弃。
  const [sections, setSections] = useState<career.EditableMaterialSection[]>([]);
  const [draftNotice, setDraftNotice] = useState('');
  const [reviseErrCode, setReviseErrCode] = useState<string>();
  const [fromTimeline, setFromTimeline] = useState(false);
  // 材料域回读：修订的持久事实在材料草稿里（Web PreparationPage 同语义——回执只是回声）。
  const [revisedBody, setRevisedBody] = useState<MaterialBody>();

  const pendingProgress = career.pendingProgressWrite();
  const pendingPreparation = career.pendingPreparationWrite();
  const pendingMaterial = career.pendingMaterialWrite();
  const revision = desk.data?.revision;

  const loadTimeline = async (): Promise<void> => {
    setView(undefined); setViewErrCode(undefined);
    try {
      const next = await career.applicationProgress(applicationId);
      if (next.applicationId !== applicationId.trim()) throw new Error('进展读取与当前申请不匹配');
      setView(next);
    } catch (error) { setViewErrCode(typedCode(error)); throw error; }
  };
  const loadPreparations = async (): Promise<void> => {
    setPreparations((await career.listPreparations(applicationId)).preparations);
  };
  const acceptReceipt = (receipt: ProgressReceipt): void => {
    setAppendErrCode(undefined); setCorrectErrCode(undefined);
    setNote(''); setOccurredAt(''); setCorrecting(undefined);
    setProgressNotice(`已记录事件 #${receipt.seq}（${eventTypeLabels[receipt.eventType] ?? receipt.eventType}${receipt.kind === 'progress_corrected' ? '，更正' : ''}）。`);
    void loadBusy.run(loadTimeline);
  };

  // 进入某条准备草稿的修订：优先恢复本地草稿（断网期间保留的编辑，heading/content），
  // claims 一律以服务端正文为准回填——本地草稿只是断网过渡态，不是第二份事实源。
  const startEditing = (receipt: PreparationReceipt): void => {
    setEditing(receipt);
    setReviseErrCode(undefined);
    const server = career.editableFromBody(receipt.body);
    const local = readPreparationDraft(receipt.applicationId, receipt.focus);
    if (local && local.materialId === receipt.materialId) {
      setSections(server.map((section, index) => ({ ...section, ...(local.sections[index] ? { heading: local.sections[index].heading, content: local.sections[index].content } : {}) })));
      setDraftNotice(`已恢复本地草稿（保存于 ${formatTime(local.savedAt)}，尚未提交）：可继续编辑，联网后请显式保存修订。`);
      return;
    }
    setSections(server);
    setDraftNotice('');
  };

  const draftFromSections = (): PreparationDraftRecord => ({
    applicationId: editing!.applicationId, focus: editing!.focus,
    ...(editing!.preparationId ? { preparationId: editing!.preparationId } : {}),
    ...(editing!.materialId ? { materialId: editing!.materialId } : {}),
    sections: sections.map(section => ({ heading: section.heading, content: section.content })),
    savedAt: new Date().toISOString(), // savePreparationDraft 会以实际写入时间覆盖
  });

  const saveRevision = async (): Promise<void> => {
    if (!editing?.materialId) throw new Error('该准备尚未物化为材料草稿，无法在本端修订');
    // 断网语义：先保留本地草稿（可再编辑），再尝试提交；失败也不静默改申请状态。
    savePreparationDraft(draftFromSections());
    const body = career.bodyFromEditable(sections);
    try {
      await career.editMaterial({ materialId: editing.materialId, body });
      clearPreparationDraft(editing.applicationId, editing.focus);
      setDraftNotice(''); setReviseErrCode(undefined);
      setGenNotice('准备草稿修订已提交（仍是可审阅草稿，发布需另行确认材料版本）。');
      // 与 Web 同语义：回执只是回声，修订的持久事实从材料域回读。
      setRevisedBody((await career.material(editing.materialId)).body);
      void listBusy.run(loadPreparations);
    } catch (error) {
      setReviseErrCode(typedCode(error));
      if (looksOffline(error)) setDraftNotice('网络不可用：已保留本地草稿（可继续编辑，未提交）。申请进展没有任何改动；联网后请再点「保存修订」显式同步，不会自动提交。');
      throw error;
    }
  };

  return <Screen title='申请进展与准备'>
    <Text className='wk-display'>一条权威时间线，{'\n'}一份锚定投递版的准备。</Text>
    <Notice tone='info'>与 Web 端同一后端合同：事件按服务端权威顺序呈现，纠错追加引用事件、原文保留可追溯；面试准备基于你实际投递的版本生成，未确认投递时只提示、绝不自行改用最新材料版本。</Notice>

    {/* 恢复态置顶（T24/T26/T32 教训）：结果未知的写入第一屏可见、可操作。 */}
    {pendingProgress && <>
      <Notice tone='warning'>有一次结果未知的进展写入（{pendingProgress.requestId.slice(0, 10)}…）。请先用原请求对账，不会重复执行。</Notice>
      <Action secondary loading={recProgBusy.busy} onClick={() => void recProgBusy.run(async () => { acceptReceipt(await career.reconcilePendingProgress()); })}>用原请求对账进展</Action>
      {recProgBusy.error && <Notice tone='danger'>{recProgBusy.error} 对账被拒时说明该请求不存在或不属于当前空间；可再用原编号重试。</Notice>}
      <Action secondary loading={retryProgBusy.busy} onClick={() => void retryProgBusy.run(async () => { acceptReceipt(await career.retryPendingProgress()); })}>用原请求编号重试进展写入</Action>
      {retryProgBusy.error && <Notice tone='danger'>{retryProgBusy.error} 重试沿用原请求编号与原事件计数，服务端幂等不会重复执行。</Notice>}
    </>}
    {pendingPreparation && <>
      <Notice tone='warning'>有一次结果未知的准备生成（{pendingPreparation.requestId.slice(0, 10)}…）。请先用原请求对账，不会重复生成。</Notice>
      <Action secondary loading={recPrepBusy.busy} onClick={() => void recPrepBusy.run(async () => { await career.reconcilePendingPreparation(); setGenNotice('已对账到准备回执。'); void listBusy.run(loadPreparations); })}>用原请求对账准备</Action>
      {recPrepBusy.error && <Notice tone='danger'>{recPrepBusy.error} 对账被拒时说明该请求不存在或不属于当前空间；可再用原编号重试。</Notice>}
      <Action secondary loading={retryPrepBusy.busy} onClick={() => void retryPrepBusy.run(async () => {
        try {
          await career.retryPendingPreparation();
          setGenErrCode(undefined); setGenNotice('准备生成已恢复完成。');
          void listBusy.run(loadPreparations);
        } catch (error) {
          if (career.isPreparationVersionUnknown(error)) {
            // 与生成按钮同一 typed 提示态：恢复链路绝不静默改用最新材料版本。
            setGenErrCode(career.PREPARATION_VERSION_UNKNOWN);
            setGenNotice('未确认实际投递版本：此申请还没有已确认的投递版本，系统不会自行改用最新材料版本。请先在「申请与材料」页记录投递（绑定实际投递的版本，或显式选择未知口径），再从这里用原请求编号重试。');
          }
          throw error;
        }
      })}>用原请求编号重试准备生成</Action>
      {retryPrepBusy.error && <Notice tone='danger'>{retryPrepBusy.error} 重试沿用原请求编号，服务端幂等不会重复执行。</Notice>}
    </>}
    {pendingMaterial && <>
      <Notice tone='warning'>有一次结果未知的材料写入（{pendingMaterial.requestId.slice(0, 10)}…）。保存准备修订走同一恢复链；重发沿用原请求编号与原正文，服务端幂等不会重复执行。</Notice>
      <Action secondary loading={retryMatBusy.busy} onClick={() => void retryMatBusy.run(async () => {
        const receipt = await career.retryPendingMaterial();
        setGenNotice('材料修订已用原请求编号恢复完成（幂等重放，不会重复写入）。');
        // 与保存后同语义：修订的持久事实从材料域回读。
        try { setRevisedBody((await career.material(receipt.materialId)).body); } catch { /* 回读失败不掩埋重试成功的事实 */ }
      })}>用原请求编号重试材料修订</Action>
      {retryMatBusy.error && <Notice tone='danger'>{retryMatBusy.error} 重试被拒时说明该请求编号已对应其他内容或不属于当前空间；可到「申请与材料」页对账。</Notice>}
    </>}

    <Card>
      <Text className='wk-h3'>申请进展时间线</Text>
      <Field label='申请编号' value={applicationId} onChange={setApplicationId} placeholder='申请编号（可从「申请与材料」页带入）' />
      <Action secondary loading={loadBusy.busy} onClick={() => void loadBusy.run(loadTimeline)}>读取进展</Action>
      {loadBusy.error && <Notice tone='danger'>{loadBusy.error}{viewErrCode === 'forbidden' ? ' 当前空间不可访问此申请的进展。' : viewErrCode === 'not_found' ? ' 未找到此申请（可能不属于当前空间）。可核对编号后重试。' : ' 可稍后重试。'}</Notice>}
      {progressNotice && <Notice tone='info'>{progressNotice}</Notice>}
      {view && <>
        <View className='wk-between'>
          <Badge tone='info'>当前阶段：{stageLabels[view.stage] ?? view.stage}</Badge>
          <Text className='wk-muted wk-small'>事件 {view.events.length} 条 · 修订 {view.revision}</Text>
        </View>
        <Notice tone='info'>七阶段投影由全部事件确定性推导；筛选、刷新、重进始终一致。</Notice>
        {view.events.map(event => <View key={event.eventId} className='wk-listrow'>
          <View className='wk-grow'>
            <Text className='wk-row-title'>#{event.seq} {eventTypeLabels[event.eventType] ?? event.eventType}{event.note ? `：${event.note}` : ''}</Text>
            <Text className='wk-muted wk-small'>{formatTime(event.occurredAt)} · 来源：{sourceKindLabels[event.source.kind] ?? event.source.kind}{event.source.label ? `（${event.source.label}）` : ''} · 确认者 {event.confirmer}</Text>
            {event.kind === 'progress_corrected' ? <Text className='wk-muted wk-small'>更正事件（更正 {event.correctsEventId}）</Text> : null}
            {event.corrected ? <Text className='wk-muted wk-small'>已更正：原文保留，投影采用更正语义</Text> : null}
            <View className='wk-between'>
              {event.eventType === 'interview' ? <Text className='wk-small' onClick={() => { setFocus('interview_prep'); setFromTimeline(true); try { void Taro.pageScrollTo({ selector: '#wk-preparation', duration: 300 }); } catch { /* 滚动失败不阻断入口 */ } }}>为这场面试做准备 ›</Text> : <Text />}
              {event.kind === 'progress_appended' ? <Text className='wk-small' onClick={() => { setCorrecting(event); setEventType(event.eventType); setNote(''); }}>纠错 ›</Text> : null}
            </View>
          </View>
        </View>)}
      </>}
    </Card>

    {view && <Card>
      <Text className='wk-h3'>{correcting ? `纠错事件 #${correcting.seq}（追加更正，原文保留）` : '录入一件进展事件'}</Text>
      <Text className='wk-muted wk-small'>事件类型</Text>
      {EVENT_TYPES.map(type => <Text key={type} className='wk-small' onClick={() => setEventType(type)}>{eventType === type ? '● ' : '○ '}{eventTypeLabels[type]}</Text>)}
      <Field label='备注（可选）' value={note} onChange={setNote} placeholder='例如：一面定在 10 月 8 日下午' />
      <Field label='声明发生时间（格式 2026-09-25 20:00；留空即记录确认时间）' value={occurredAt} onChange={setOccurredAt} placeholder='2026-09-25 20:00' />
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel={correcting ? '提交更正' : '录入进展'} customStyle={tdesignButtonStyle} loading={correcting ? correctBusy.busy : appendBusy.busy} onTap={() => void (correcting ? correctBusy : appendBusy).run(async () => {
          // 声明时间严格解析（与投递确认同一判据）：无效输入显式拒绝，绝不静默丢弃。
          const parsed = career.parseDeclaredOccurredAt(occurredAt);
          if (parsed.status === 'invalid') throw new Error('声明时间格式无效：请使用 2026-09-25 20:00 这样的本地时间，或留空');
          const input = { applicationId: applicationId.trim(), eventType, ...(note.trim() ? { note: note.trim() } : {}), ...(parsed.status === 'ok' ? { occurredAt: parsed.iso } : {}) };
          try {
            if (correcting) { acceptReceipt(await career.correctProgressEvent({ ...input, correctsEventId: correcting.eventId }, view.revision)); setCorrectErrCode(undefined); }
            else { acceptReceipt(await career.appendProgressEvent(input, view.revision)); setAppendErrCode(undefined); }
          } catch (error) { (correcting ? setCorrectErrCode : setAppendErrCode)(typedCode(error)); throw error; }
        })}>{correcting ? '提交更正（追加引用事件）' : '录入进展'}</t-button>
      </View></View>
      {correcting && <Action secondary onClick={() => setCorrecting(undefined)}>取消纠错</Action>}
      {(appendBusy.error || correctBusy.error) && <Notice tone='danger'>{appendBusy.error ?? correctBusy.error}{(appendErrCode ?? correctErrCode) === 'revision_conflict' ? ' 事件已被其他端先行记录：请重新读取进展后再次录入（新录入会使用新的请求编号）。' : (appendErrCode ?? correctErrCode) === 'progress_event_not_found' ? ' 被更正的事件不存在或不属于此申请，请重新读取。' : ''}</Notice>}
      {(appendErrCode ?? correctErrCode) === 'revision_conflict' && <Action secondary onClick={() => void loadBusy.run(loadTimeline)}>重新读取进展</Action>}
    </Card>}

    <Card>
      <View id='wk-preparation'>
        <Text className='wk-h3'>按需准备（面试准备 / 求职信）</Text>
      </View>
      {fromTimeline && <Notice tone='info'>已从时间线进入：将为这场面试准备（基于该申请实际投递的版本）。</Notice>}
      <Text className='wk-muted wk-small'>准备焦点</Text>
      {(Object.keys(focusLabels) as PreparationFocus[]).map(option => <Text key={option} className='wk-small' onClick={() => setFocus(option)}>{focus === option ? '● ' : '○ '}{focusLabels[option]}</Text>)}
      <DataBoundary state={desk}>{loaded => loaded && <Text className='wk-muted wk-small'>当前档案修订 {loaded.revision}（准备生成按此修订提交）</Text>}</DataBoundary>
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='生成准备草稿' customStyle={tdesignButtonStyle} loading={genBusy.busy} disabled={!applicationId.trim() || revision === undefined} onTap={() => void genBusy.run(async () => {
          try {
            await career.generatePreparation({ applicationId: applicationId.trim(), focus });
            setGenErrCode(undefined); setGenNotice('');
            void listBusy.run(loadPreparations);
          } catch (error) {
            setGenErrCode(typedCode(error));
            if (career.isPreparationVersionUnknown(error)) {
              // typed 提示态（与 Web 同语义）：绝不静默改用最新材料版本。
              setGenNotice('未确认实际投递版本：此申请还没有已确认的投递版本，系统不会自行改用最新材料版本。请先在「申请与材料」页记录投递（绑定实际投递的版本，或显式选择未知口径），再生成准备。');
            }
            throw error;
          }
        })}>生成准备草稿</t-button>
      </View></View>
      {genBusy.error && <Notice tone='danger'>{genBusy.error}{genErrCode === 'revision_conflict' ? ' 档案已更新：重新读取修订后再生成（新生成会使用新的请求编号）。' : genErrCode === 'preparation_generation_failed' ? ' 生成失败但请求已保留：可用原请求编号恢复重试，不会留下空白成功产物。' : ''}</Notice>}
      {genNotice && <Notice tone={genErrCode === 'preparation_version_unknown' ? 'warning' : 'info'}>{genNotice}</Notice>}
      <Action secondary loading={listBusy.busy} onClick={() => void listBusy.run(loadPreparations)}>{preparations ? '刷新准备列表' : '查看已有准备'}</Action>
      {listBusy.error && <Notice tone='danger'>{listBusy.error} 可稍后重试。</Notice>}
      {(preparations ?? []).map(item => <View key={item.preparationId} className='wk-listrow'>
        <View className='wk-grow'>
          <Text className='wk-row-title'>{focusLabels[item.focus] ?? item.focus} · {item.status === 'draft' ? '草稿（可审阅、可修订）' : item.status === 'failed' ? `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}` : '生成中（可恢复）'}</Text>
          <Text className='wk-muted wk-small'>基于实际投递版本 V{item.anchor.version}（材料 {item.anchor.materialId.slice(0, 10)}… · 导出 {item.anchor.exportId.slice(0, 10)}… · 提交 {item.anchor.submissionId.slice(0, 10)}…）</Text>
          {item.status === 'draft' && item.sources.snapshot.snapshotId ? <Text className='wk-muted wk-small'>来源链：快照 {item.sources.snapshot.snapshotId.slice(0, 10)}…（SHA-256 {digestHead(item.sources.snapshot.snapshotSha256 ?? '')}…）· 引用事实 {item.sources.factKeys.length} 项 · 档案修订 {item.sources.profileRevision}</Text> : null}
          {item.status === 'draft' && item.body.sections.map((section, index) => <Text key={index} className='wk-muted wk-small'>{section.heading}：{section.content}</Text>)}
          {item.status === 'draft' && <Text className='wk-small' onClick={() => startEditing(item)}>修订这份草稿 ›</Text>}
        </View>
      </View>)}
    </Card>

    {editing && <Card>
      <Text className='wk-h3'>修订准备草稿（{focusLabels[editing.focus] ?? editing.focus}）</Text>
      <Text className='wk-muted wk-small'>修订走材料域同一链（claims 原样保留）；提交仍是草稿，发布需另行确认材料版本。锚定投递版本 V{editing.anchor.version} 不随修订漂移。</Text>
      {draftNotice && <Notice tone='warning'>{draftNotice}</Notice>}
      {sections.map((section, index) => <View key={index}>
        <Field label={`第 ${index + 1} 节标题`} value={section.heading} onChange={value => setSections(previous => previous.map((item, at) => at === index ? { ...item, heading: value } : item))} placeholder='例如：面试要点' />
        <Field label='内容' value={section.content} onChange={value => setSections(previous => previous.map((item, at) => at === index ? { ...item, content: value } : item))} multiline placeholder='这一节要写的内容' />
      </View>)}
      <Action secondary onClick={() => setSections(previous => [...previous, { heading: '', content: '', claims: [] }])}>新增小节</Action>
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='保存准备草稿修订' customStyle={tdesignButtonStyle} loading={reviseBusy.busy} onTap={() => void reviseBusy.run(saveRevision)}>保存修订（显式提交）</t-button>
      </View></View>
      {reviseBusy.error && <Notice tone='danger'>{reviseBusy.error}{reviseErrCode === 'outcome_unknown' ? ' 修订结果未知：本地草稿已保留，请用页首「用原请求编号重试材料修订」恢复（幂等可重放），本页不会自动重发。' : reviseErrCode === 'revision_conflict' ? ' 档案已更新：请重新读取修订后再保存（新保存会使用新的请求编号）。' : ''}</Notice>}
      <Action secondary loading={readBackBusy.busy} onClick={() => void readBackBusy.run(async () => {
        if (!editing?.materialId) throw new Error('该准备尚未物化为材料草稿');
        setRevisedBody((await career.material(editing.materialId)).body);
      })}>回读材料草稿正文（修订后的持久事实）</Action>
      {readBackBusy.error && <Notice tone='danger'>{readBackBusy.error} 可稍后重试。</Notice>}
      <Action secondary onClick={() => { setEditing(undefined); setSections([]); setDraftNotice(''); setRevisedBody(undefined); }}>结束修订</Action>
    </Card>}

    {revisedBody && <Card tone='mint'>
      <Text className='wk-h3'>材料域回读（修订后的草稿正文）</Text>
      {revisedBody.sections.map((section, index) => <Text key={index} className='wk-muted wk-small'>{section.heading}：{section.content}</Text>)}
      <Text className='wk-muted wk-small'>回执只是回声：修订的持久事实在材料草稿里（Web PreparationPage 同语义）；仍未确认成版本，可继续修订。</Text>
    </Card>}

    <Notice tone='info'>断网时只保留本地草稿（按账号隔离，登出即清），申请进展绝不本地改写；联网后一切同步都需要你显式确认。结果未知的写入都可以用原请求编号对账或安全重发。</Notice>
  </Screen>;
}
