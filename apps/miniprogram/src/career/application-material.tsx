import { useRef, useState } from 'react';
import Taro from '@tarojs/taro';
import { Text, View } from '@tarojs/components';
import { Screen, Card, Action, Field, Notice, Badge, Empty, DataBoundary, useData, useAction, useSession } from '../components/ui.tsx';
import * as career from '../services/career.ts';
import type { ApplicationReceipt, MaterialExportReceipt, MaterialView, SubmissionChannel, SubmissionReceipt } from '../../../../packages/api-client/src/career.ts';
import type { Evaluation } from '../../../../packages/career-core/src/contracts.ts';
import type { ExportOpenRecord } from '../adapters/career-platform.ts';
import { formatTime } from '../core/format.ts';
import { errorMessage } from '../core/errors.ts';

// T26 申请、材料与本人投递：与 Web 同一后端合同（同源同版本）。硬条件不符警示常驻、
// 显式继续才可申请；材料确认生成不可变新版本；导出双格式经授权认证兑付下载并做全字节
// digest 校验；投递只记录用户声明的渠道/时间/版本绑定——零自动提交、零外发。
const CHANNELS: { value: SubmissionChannel; label: string }[] = [
  { value: 'web', label: '招聘网站' },
  { value: 'email', label: '邮件' },
  { value: 'other', label: '其他渠道' },
];
const evaluationLabel: Record<string, string> = { eligible: '符合已识别条件', ineligible: '不符合（硬性冲突）', unknown: '待确认' };
const evaluationTone: Record<string, 'success' | 'danger' | 'warning'> = { eligible: 'success', ineligible: 'danger', unknown: 'warning' };
const linkStateLabel: Record<string, string> = { linking: 'Task 关联中（可对账恢复）', ready: 'Task 已就绪', link_failed: 'Task 关联失败' };
const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
const digestHead = (digest: string): string => digest.slice(0, 12);
const typedCode = (error: unknown): string | undefined => { const code = (error as { code?: unknown } | null | undefined)?.code; return typeof code === 'string' ? code : undefined; };

export default function ApplicationMaterialPage() {
  const session = useSession();
  const desk = useData(`career:${session.userId}:${session.tenantId}`, () => career.loadCareer());
  const evaluateBusy = useAction(); const applyBusy = useAction(); const linkBusy = useAction();
  const matLoadBusy = useAction(); const matEditBusy = useAction(); const matConfirmBusy = useAction();
  const publishBusy = useAction(); const downloadBusy = useAction(); const submitBusy = useAction(); const listBusy = useAction();
  const recoverAppBusy = useAction(); const recoverMatBusy = useAction(); const recoverSubBusy = useAction();
  const recoverPubBusy = useAction(); const retryPubBusy = useAction();

  const params = Taro.getCurrentInstance().router?.params as Record<string, string | undefined> | undefined;
  const [opportunityId, setOpportunityId] = useState(params?.opportunityId ?? '');
  const [snapshotId, setSnapshotId] = useState(params?.snapshotId ?? '');
  const [evaluation, setEvaluation] = useState<Evaluation>();
  const [batchIdentity, setBatchIdentity] = useState('');
  const [acknowledged, setAcknowledged] = useState(false);
  const [application, setApplication] = useState<ApplicationReceipt>();
  const [appErrCode, setAppErrCode] = useState<string>();
  const [materialId, setMaterialId] = useState('');
  // CAREER-OCR H4：「读取材料」在途时编号可能被改（onChange 只失效后续新读取），
  // 完成时必须与最新输入核对，绝不把 A 的正文/编辑态落进 B 名下。
  const materialIdRef = useRef('');

  const [material, setMaterial] = useState<MaterialView>();
  // F4：多节全量编辑模型——读取时载入全部小节与 claims，保存原样回传（绝不静默丢弃）。
  const [sections, setSections] = useState<career.EditableMaterialSection[]>([]);
  const [confirmedVersion, setConfirmedVersion] = useState<number>();
  const [exports, setExports] = useState<MaterialExportReceipt[]>();
  const [checks, setChecks] = useState<ExportOpenRecord[]>([]);
  const [channel, setChannel] = useState<SubmissionChannel>('web');
  const [occurredAt, setOccurredAt] = useState('');
  const [versionChoice, setVersionChoice] = useState('');
  const [note, setNote] = useState('');
  const [submissions, setSubmissions] = useState<SubmissionReceipt[]>();
  const [subErrCode, setSubErrCode] = useState<string>();
  // OCR r3 ocr3-029/030：未对账封锁把新写入挡在门外时，显式放弃是唯一解除出口；
  // setAbandonNotice 同时触发重渲染让 pending* 重读。
  const [abandonNotice, setAbandonNotice] = useState('');

  const pendingApplication = career.pendingApplication();
  const pendingEvaluation = career.pendingEvaluation();
  const pendingMaterial = career.pendingMaterialWrite();
  const pendingSubmission = career.pendingSubmission();
  const pendingPublish = career.pendingMaterialPublish();
  const ineligible = evaluation?.status === 'ineligible';
  // 硬条件警示常驻：评估不符合时创建前常驻；申请回执带 warning 时存续期间常驻。
  const hardWarning = (ineligible || application?.warning) && (
    <Notice tone='danger'>硬性条件不符（警示常驻）：所选评估结论为“不符合”。默认阻断申请；勾选显式继续后可提交，但该申请不计入合格申请指标{application?.warning?.reasonCode ? `（原因 ${application.warning.reasonCode}）` : ''}。</Notice>
  );

  const loadExports = async (id: string): Promise<void> => setExports((await career.listMaterialExports(id)).exports);
  const loadSubmissions = async (applicationId: string): Promise<void> => setSubmissions((await career.listSubmissions(applicationId)).submissions);

  return <Screen title='申请与材料'>
    <Text className='wk-display'>一份申请，一份材料，{'\n'}一次本人投递。</Text>
    <Notice tone='info'>与 Web 端同源同版本：同一份档案、同一份申请与结构化正文；材料确认后生成不可变新版本，旧版本不被覆盖。</Notice>

    {/* 恢复态置顶（T24 discovery 同款次序）：结果未知的写入是当务之急，必须第一屏可见、可操作。 */}
    {pendingEvaluation && <><Notice tone='warning'>有一次结果未知的资格评估（{pendingEvaluation.requestId.slice(0, 10)}…），在恢复前不会发起另一次评估。</Notice>
      <Action secondary loading={evaluateBusy.busy} onClick={() => void evaluateBusy.run(async () => setEvaluation(await career.reconcilePendingEvaluation()))}>对账原评估</Action>
      <Action secondary onClick={() => void evaluateBusy.run(async () => setEvaluation(await career.retryPendingEvaluation()))}>安全重发原评估</Action></>}
    {pendingApplication && <>
      <Notice tone='warning'>有一次结果未知的申请创建（{pendingApplication.requestId.slice(0, 10)}…）。请先用原请求对账，不会重复创建。</Notice>
      <Action secondary loading={recoverAppBusy.busy} onClick={() => void recoverAppBusy.run(async () => { setApplication(await career.reconcilePendingApplication()); })}>用原请求对账申请</Action>
      {recoverAppBusy.error && <Notice tone='danger'>{recoverAppBusy.error} 对账被拒时说明该请求不属于当前空间或不存在；可稍后再试，不会自动重发。</Notice>
      }
      <Action secondary onClick={() => { career.abandonPendingApplication(); setAbandonNotice('已放弃本次申请恢复：创建入口恢复可用。若原申请实际已生效，以服务端记录为准——重新创建会收到「此岗位与该批次已存在申请」提示。'); }}>放弃本次申请恢复</Action>
    </>}
    {pendingMaterial && <>
      <Notice tone='warning'>有一次结果未知的材料写入（{pendingMaterial.requestId.slice(0, 10)}…）。</Notice>
      <Action secondary loading={recoverMatBusy.busy} onClick={() => void recoverMatBusy.run(async () => {
        // 回执携带权威 materialId（新建材料写入失败未知时输入框仍为空）：以回执为准回读，
        // 绝不用页面输入态读材料（OCR high-10）。
        const receipt = await career.reconcilePendingMaterial();
        setMaterialId(receipt.materialId);
        const view = await career.material(receipt.materialId);
        setMaterial(view);
        setSections(career.editableFromBody(view.body));
        setConfirmedVersion(undefined);
        setExports(view.versionCount > 0 ? (await career.listMaterialExports(receipt.materialId)).exports : undefined);
      })}>用原请求对账材料</Action>
      {recoverMatBusy.error && <Notice tone='danger'>{recoverMatBusy.error} 对账被拒时说明该请求不属于当前空间或不存在；intent 保留，可稍后再试。</Notice>
      }
      <Action secondary onClick={() => { career.abandonPendingMaterialWrite(); setAbandonNotice('已放弃本次材料写入恢复：编辑与确认入口恢复可用。若原写入实际已生效，重新读取材料即可见最新版本。'); }}>放弃本次材料恢复</Action>
    </>}
    {abandonNotice && <Notice tone='info'>{abandonNotice}</Notice>}

    {pendingSubmission && <>
      <Notice tone='warning'>有一次结果未知的投递确认（{pendingSubmission.requestId.slice(0, 10)}…）。请先对账，确认前不会重复记录。</Notice>
      <Action secondary loading={recoverSubBusy.busy} onClick={() => void recoverSubBusy.run(async () => {
        const receipt = await career.reconcilePendingSubmission();
        if (application) await loadSubmissions(application.applicationId);
        void receipt;
      })}>用原请求对账投递</Action>
      {recoverSubBusy.error && <Notice tone='danger'>{recoverSubBusy.error} 对账被拒时说明该请求不属于当前空间或不存在；intent 保留，可稍后再试。</Notice>
      }
      <Action secondary onClick={() => { career.abandonPendingSubmission(); setAbandonNotice('已放弃本次投递确认恢复：记录入口恢复可用。若原确认实际已生效，读取投递记录即可见（一申请一记录，重复记录会被服务端拒绝）。'); }}>放弃本次投递恢复</Action>
    </>}

    <DataBoundary state={desk}>{view => view && <Card>
      <Text className='wk-muted wk-small'>当前档案修订 {view.revision}（申请与材料写入将按此修订校验）</Text>
      <Text className='wk-muted wk-small'>待确认事实 {view.proposals.length} 条 · 已确认 {view.facts.length} 条</Text>
    </Card>}</DataBoundary>

    <Card>
      <Text className='wk-h3'>岗位与资格评估</Text>
      <Field label='岗位编号' value={opportunityId} onChange={setOpportunityId} placeholder='从分享导入后自动带入，或粘贴岗位编号' />
      <Field label='快照编号' value={snapshotId} onChange={setSnapshotId} placeholder='岗位快照编号' />
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='发起资格评估' customStyle={tdesignButtonStyle} loading={evaluateBusy.busy} onTap={() => void evaluateBusy.run(async () => {
          setEvaluation(await career.evaluateOpportunity(opportunityId, snapshotId));
        })}>用当前档案评估</t-button>
      </View></View>
      {evaluateBusy.error && <Notice tone='danger'>{evaluateBusy.error} 评估未完成时不会创建任何申请。</Notice>}
      {evaluation && <View className='wk-listrow'>
        <View className='wk-grow'>
          <Text className='wk-row-title'>评估 {evaluation.evaluationId}</Text>
          <Text className='wk-muted wk-small'>按档案修订 {evaluation.profileRevision} · 岗位 {evaluation.opportunityId} · 快照 {evaluation.snapshotId}</Text>
        </View>
        <Badge tone={evaluationTone[evaluation.status] ?? 'neutral'}>{evaluationLabel[evaluation.status] ?? evaluation.status}</Badge>
      </View>}
      {evaluation && <View>
        <Text className='wk-row-title'>硬性条件证据</Text>
        {evaluation.hard.rules.map(rule => <View key={rule.ruleId} className='wk-listrow'><View className='wk-grow'>
          <Text>{rule.criterion}：{evaluationLabel[rule.outcome] ?? rule.outcome}（{rule.reasonCode}）</Text>
          {rule.jobEvidence && <Text className='wk-muted wk-small'>职位原文：“{rule.jobEvidence.quotedText}”</Text>}
          {rule.profileEvidence && <Text className='wk-muted wk-small'>档案事实：{rule.profileEvidence.factKey}：{rule.profileEvidence.value}（修订 {rule.profileEvidence.factRevision}）</Text>}
        </View></View>)}
        <Text className='wk-row-title'>匹配与差距</Text>
        {evaluation.soft.matches.map((match, index) => <Text key={`${match.kind}-${index}`} className='wk-muted wk-small'>匹配 {match.kind}：{match.value}；职位证据“{match.jobEvidence.quotedText}”，档案事实 {match.profileEvidence.factKey}：{match.profileEvidence.value}</Text>)}
        {!evaluation.soft.matches.length && <Text className='wk-muted wk-small'>当前没有可引用的支持性匹配事实。</Text>}
        {evaluation.facts.map(fact => <Text key={`${fact.factKey}-${fact.factRevision}`} className='wk-muted wk-small'>纳入评估的档案事实：{fact.factKey}：{fact.value}</Text>)}
      </View>}
    </Card>

    {evaluation && <Card>
      <Text className='wk-h3'>创建求职申请</Text>
      {hardWarning}
      {ineligible && <View className='wk-listrow' onClick={() => setAcknowledged(!acknowledged)}>
        <View className='wk-grow'><Text className={acknowledged ? 'wk-row-title' : 'wk-muted'}>{acknowledged ? '☑' : '☐'} 我已知晓硬性条件不符，仍要显式继续申请</Text></View>
      </View>}
      <Field label='招聘批次标识（同一岗位不同批次可分别申请）' value={batchIdentity} onChange={setBatchIdentity} placeholder='例如：2026 秋招 A 批' />
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='创建求职申请' customStyle={tdesignButtonStyle} loading={applyBusy.busy} disabled={pendingApplication !== null || (ineligible && !acknowledged)} onTap={() => void applyBusy.run(async () => {
          try {
            const receipt = await career.createApplication({ opportunityId, snapshotId, evaluationId: evaluation.evaluationId, batchIdentity, continueDespiteHardFailure: ineligible && acknowledged });
            setApplication(receipt);
            setAppErrCode(undefined);
            setMaterialId('');
            setMaterial(undefined);
            setSections([]);
            setConfirmedVersion(undefined);
            setExports(undefined);
            setChecks([]);
            setSubmissions(undefined);
            setVersionChoice('');
          } catch (error) { setAppErrCode(typedCode(error)); throw error; }
        })}>创建申请</t-button>
      </View></View>
      {applyBusy.error && <Notice tone='danger'>{applyBusy.error}{appErrCode === 'revision_conflict' ? ' 档案已更新：下拉刷新读取最新修订后重试（新提交将使用新请求编号）。' : appErrCode === 'application_conflict' ? ' 此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task；可为其他批次创建。' : appErrCode === 'hard_ineligible_requires_continue' ? ' 硬性条件不符，需要先勾选显式继续才能提交申请。' : ''}</Notice>}
    </Card>}

    {application && <Card tone={application.qualified ? 'mint' : 'warning'}>
      <Text className='wk-h3'>申请回执</Text>
      <View className='wk-between'>
        <Badge tone={application.linkState === 'ready' ? 'success' : application.linkState === 'linking' ? 'warning' : 'danger'}>{linkStateLabel[application.linkState]}</Badge>
        <Badge tone={application.qualified ? 'success' : 'danger'}>{application.qualified ? '合格申请' : '不合格申请（显式继续）'}</Badge>
      </View>
      <Text className='wk-small'>申请 {application.applicationId} · 请求 {application.requestId}</Text>
      {application.taskId && <Text className='wk-muted wk-small'>Task {application.taskId}</Text>}
      <Text className='wk-muted wk-small'>固定证据：快照 {application.pinnedEvidence.snapshotId} · 评估 {application.pinnedEvidence.evaluationId} · 档案修订 {application.pinnedEvidence.profileRevision} · 批次 {application.pinnedEvidence.batchIdentity}</Text>
      {application.warning && hardWarning}
      {application.linkState !== 'ready' && <Action secondary loading={linkBusy.busy} onClick={() => void linkBusy.run(async () => { setApplication(await career.reconcileApplicationLink(application.requestId)); })}>用原请求编号对账 Task 关联</Action>}
      {linkBusy.error && <Notice tone='danger'>{linkBusy.error} 申请与固定证据已保留，可稍后再对账。</Notice>}
      <Action secondary onClick={() => { setApplication(undefined); setAcknowledged(false); setBatchIdentity(''); }}>为其他批次创建新申请</Action>
    </Card>}

    {(application || materialId) && <Card>
      <Text className='wk-h3'>结构化正文材料</Text>
      <Field label='材料编号（创建后自动带入；重进页面可粘贴读取）' value={materialId} onChange={value => {
        setMaterialId(value); materialIdRef.current = value;
        // 编号一旦改动，已读取的正文/版本/导出/校验即失效——绝不把 A 的 sections 写进
        // B 名下（OCR high-11）。保存草稿成功后写入的编号走 setMaterialId 原始 setter，
        // 不经过这里的失效逻辑。
        setMaterial(undefined);
        setSections([]);
        setConfirmedVersion(undefined);
        setExports(undefined);
        setChecks([]);
      }} placeholder='材料编号' />
      <Action secondary loading={matLoadBusy.busy} onClick={() => void matLoadBusy.run(async () => {
        materialIdRef.current = materialId;
        const view = await career.material(materialId);
        if (view.materialId !== materialIdRef.current.trim()) return; // 读取在途时编号已改：丢弃过期响应
        setMaterial(view);
        // F4：全量载入编辑态——每个小节与全部 claims 进入编辑模型，保存原样回传。
        setSections(career.editableFromBody(view.body));
        // confirmedVersion 是“本端对这份材料确认过哪个版本”的状态，绝不跨材料残留
        // （OCR med-46：A 的确认版本不得固化成 B 的不可变导出）。
        setConfirmedVersion(undefined);
        setExports(view.versionCount > 0 ? (await career.listMaterialExports(materialId)).exports : undefined);
      })}>读取材料</Action>
      {matLoadBusy.error && <Notice tone='danger'>{matLoadBusy.error} 若材料不属于当前空间或不存在，会如实提示；不会用演示数据代替。</Notice>}
      {material && <>
        <Text className='wk-muted wk-small'>状态 {material.status} · 已确认版本 {material.versionCount} 个（修改只新增版本，不覆盖旧版）</Text>
        {!!material.reviewRisks.length && <Notice tone='warning'>待核风险（如实展示，不虚构）：{material.reviewRisks.map(risk => risk.message).join('；')}</Notice>}
        <Text className='wk-muted wk-small'>版本历史：{material.versions.map(version => `V${version.version}`).join(' · ')}</Text>
      </>}
      <Text className='wk-h3'>小节编辑（保存时全部小节与主张原样提交，不会丢弃）</Text>
      {sections.map((section, index) => <View key={`section-${index}`} className='wk-listrow'>
        <View className='wk-grow'>
          <Field label={`第 ${index + 1} 节标题`} value={section.heading} onChange={value => setSections(previous => previous.map((item, at) => at === index ? { ...item, heading: value } : item))} placeholder='例如：教育背景' />
          <Field label='内容' value={section.content} onChange={value => setSections(previous => previous.map((item, at) => at === index ? { ...item, content: value } : item))} multiline placeholder='这一节要写的内容' />
          <Text className='wk-muted wk-small'>主张 {section.claims.length} 条（本端不改主张，保存时原样保留）</Text>
          <Action secondary onClick={() => setSections(previous => previous.filter((_, at) => at !== index))}>删除本节</Action>
        </View>
      </View>)}
      <Action secondary onClick={() => setSections(previous => [...previous, { heading: '', content: '', claims: [] }])}>添加小节</Action>
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='保存材料草稿' customStyle={tdesignButtonStyle} loading={matEditBusy.busy} disabled={pendingMaterial !== null} onTap={() => void matEditBusy.run(async () => {
          const body = career.bodyFromEditable(sections);
          const receipt = material
            ? await career.editMaterial({ materialId, body })
            : await career.editMaterial({ opportunityId: application?.pinnedEvidence.opportunityId ?? opportunityId, snapshotId: application?.pinnedEvidence.snapshotId ?? snapshotId, body });
          setMaterialId(receipt.materialId);
          setMaterial(await career.material(receipt.materialId));
        })}>保存草稿</t-button>
      </View></View>
      {matEditBusy.error && <Notice tone='danger'>{matEditBusy.error} 冲突时请重读材料后重试。</Notice>}
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='default' ariaLabel='确认材料新版本' customStyle={tdesignButtonStyle} loading={matConfirmBusy.busy} disabled={pendingMaterial !== null} onTap={() => void matConfirmBusy.run(async () => {
          const receipt = await career.confirmMaterial(materialId);
          setConfirmedVersion(receipt.version);
          setMaterial(await career.material(materialId));
        })}>确认为不可变新版本</t-button>
      </View></View>
      {confirmedVersion !== undefined && <Notice tone='info'>已确认版本 V{confirmedVersion}：此后发布与投递都绑定这个确定版本。</Notice>}
      {matConfirmBusy.error && <Notice tone='danger'>{matConfirmBusy.error}</Notice>}
    </Card>}

    {(((material?.versionCount ?? 0) > 0) || exports || pendingPublish) && <Card>
      <Text className='wk-h3'>发布与下载（PDF / DOCX）</Text>
      {pendingPublish && <>
        <Notice tone='warning'>有一次结果未知的材料发布（{pendingPublish.requestId.slice(0, 10)}…，材料 {pendingPublish.input.materialId.slice(0, 10)}… · V{pendingPublish.input.version}）。请先用原请求对账；确认未送达后才需要安全重发，不会重复发布。</Notice>
        <Action secondary loading={recoverPubBusy.busy} onClick={() => void recoverPubBusy.run(async () => {
          const receipt = await career.reconcilePendingMaterialPublish();
          if (!receipt) throw Object.assign(new Error('导出列表中没有这次发布的记录：请求可能未送达'), { code: 'publish_receipt_missing' });
          setMaterialId(receipt.materialId);
          setConfirmedVersion(receipt.version);
          await loadExports(receipt.materialId);
        })}>用原请求对账发布</Action>
        {recoverPubBusy.error && <Notice tone='danger'>{recoverPubBusy.error} intent 保留：可稍后再对账，或用原请求编号安全重发。</Notice>}
        <Action secondary loading={retryPubBusy.busy} onClick={() => void retryPubBusy.run(async () => {
          const receipt = await career.retryPendingMaterialPublish();
          setMaterialId(receipt.materialId);
          setConfirmedVersion(receipt.version);
          await loadExports(receipt.materialId);
        })}>用原请求编号安全重发发布</Action>
        {retryPubBusy.error && <Notice tone='danger'>{retryPubBusy.error} 重发沿用原请求编号与原版本（服务端幂等不会重复执行）；结果仍未知时 intent 保留，可再次对账。</Notice>
        }
        <Action secondary onClick={() => { career.abandonPendingMaterialPublish(); setAbandonNotice('已放弃本次发布恢复：发布入口恢复可用。若原发布实际已生效，刷新导出列表即可见。'); }}>放弃本次发布恢复</Action>
      </>}
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='发布双格式导出' customStyle={tdesignButtonStyle} loading={publishBusy.busy} disabled={pendingPublish !== null} onTap={() => void publishBusy.run(async () => {
          const version = confirmedVersion ?? material?.versionCount;
          if (version === undefined) throw new Error('请先确认材料版本');
          await career.publishMaterial(materialId, version);
          await loadExports(materialId);
        })}>发布 V{confirmedVersion ?? material?.versionCount ?? ''} 双格式</t-button>
      </View></View>
      {publishBusy.error && <Notice tone='danger'>{publishBusy.error}</Notice>}
      <Action secondary loading={listBusy.busy} onClick={() => void listBusy.run(async () => { await loadExports(materialId); })}>刷新导出列表</Action>
      {(exports ?? []).map(exportReceipt => <Card key={exportReceipt.exportId} tone={exportReceipt.submittable ? 'mint' : 'white'}>
        <View className='wk-between'>
          <Badge tone={exportReceipt.submittable ? 'success' : 'warning'}>{exportReceipt.submittable ? '可提交' : exportReceipt.status}</Badge>
          <Text className='wk-muted wk-small'>V{exportReceipt.version} · {exportReceipt.exportId.slice(0, 10)}…</Text>
        </View>
        <Text className='wk-muted wk-small'>正文摘要 {digestHead(exportReceipt.contentDigest)}… · {exportReceipt.files.map(file => `${file.format.toUpperCase()} ${file.verified ? '已核验' : '未核验'}`).join(' · ')}</Text>
        {exportReceipt.files.map(file => <View key={file.format} className='wk-between'>
          <View className='wk-tdesign-scope'>
            <t-button size='medium' theme='default' ariaLabel={`下载并打开 ${file.format}`} customStyle={tdesignButtonStyle} loading={downloadBusy.busy} onTap={() => void downloadBusy.run(async () => {
              const { check } = await career.openMaterialExport(materialId, exportReceipt.exportId, file.format);
              setChecks(previous => [...previous, check]);
            })}>下载并打开 {file.format.toUpperCase()}</t-button>
          </View>
        </View>)}
      </Card>)}
      {downloadBusy.error && <Notice tone='danger'>{downloadBusy.error} 授权过期会自动重取一次；仍失败时可再次点击下载。</Notice>}
      {checks.map((check, index) => <Notice key={`${check.format}-${check.checkedAt}-${index}`} tone={check.digestMatched ? 'info' : 'danger'}>
        校验记录 {check.format.toUpperCase()} V{check.version}：{check.size} 字节，全字节摘要 {digestHead(check.actualDigest)}… 与授权摘要{check.digestMatched ? '一致' : '不一致'}，{check.opened ? '已打开' : '未打开'}（{formatTime(check.checkedAt)}）。
      </Notice>)}
    </Card>}

    {application && <Card>
      <Text className='wk-h3'>投递确认（本人完成，产品只记录）</Text>
      <Notice tone='info'>投递由你本人在招聘平台完成；这里只记录你声明的渠道、时间与版本绑定，不会自动提交、发送邮件或填写任何表单。</Notice>
      {CHANNELS.map(option => <Text key={option.value} className='wk-small' onClick={() => setChannel(option.value)}>{channel === option.value ? '● ' : '○ '}{option.label}</Text>)}
      <Field label='声明投递时间（格式 2026-09-25 20:00；留空即记录确认时间）' value={occurredAt} onChange={setOccurredAt} placeholder='2026-09-25 20:00' />
      <Text className='wk-h3'>投递版本</Text>
      {(exports ?? []).filter(exportReceipt => exportReceipt.submittable).map(exportReceipt => <Text key={exportReceipt.exportId} className='wk-small' onClick={() => setVersionChoice(exportReceipt.exportId)}>{versionChoice === exportReceipt.exportId ? '● ' : '○ '}可提交导出 V{exportReceipt.version}（{exportReceipt.exportId.slice(0, 10)}…）</Text>)}
      <Text className='wk-small' onClick={() => setVersionChoice(career.SUBMISSION_VERSION_UNKNOWN_CHOICE)}>{versionChoice === career.SUBMISSION_VERSION_UNKNOWN_CHOICE ? '● ' : '○ '}版本未知（显式声明，不绑定材料版本）</Text>
      <Field label='备注（可选）' value={note} onChange={setNote} placeholder='例如：官网已投递，附职位链接' />
      {/* F1：未选择≠显式未知。一申请一记录不可逆，漏选必须阻断而不是被推断。 */}
      {!versionChoice && <Notice tone='warning'>请先在上面选择投递版本，或显式选择「版本未知」——未选择不会被当作版本未知提交。</Notice>}
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='记录投递确认' customStyle={tdesignButtonStyle} loading={submitBusy.busy} disabled={pendingSubmission !== null} onTap={() => void submitBusy.run(async () => {
          try {
            const choice = career.resolveSubmissionVersion(versionChoice, (exports ?? []).filter(exportReceipt => exportReceipt.submittable));
            if (choice.status === 'unselected') {
              setSubErrCode('submission_version_unselected');
              throw Object.assign(new Error('请先选择投递版本，或显式声明「版本未知」'), { code: 'submission_version_unselected' });
            }
            // F5：声明时间严格解析——无效输入阻断提交，绝不静默回退为确认时间。
            const declared = career.parseDeclaredOccurredAt(occurredAt);
            if (declared.status === 'invalid') {
              setSubErrCode('declared_time_invalid');
              throw Object.assign(new Error('声明投递时间无法识别（示例 2026-09-25 20:00）：请改正或留空（留空记录确认时间）'), { code: 'declared_time_invalid' });
            }
            await career.recordSubmission({
              applicationId: application.applicationId, channel,
              ...(declared.status === 'ok' ? { occurredAt: declared.iso } : {}),
              ...(choice.status === 'bound' ? { materialId: choice.materialId, exportId: choice.exportId } : {}),
              versionUnknown: choice.status === 'unknown',
              ...(note.trim() ? { note } : {}),
            });
            setSubErrCode(undefined);
            await loadSubmissions(application.applicationId);
          } catch (error) { const code = typedCode(error); if (code) setSubErrCode(code); throw error; }
        })}>记录投递</t-button>
      </View></View>
      {submitBusy.error && <Notice tone={subErrCode === 'submission_already_confirmed' ? 'warning' : 'danger'}>{submitBusy.error}{subErrCode === 'submission_already_confirmed' ? ' 此申请已有一次投递确认记录，不可重复确认。' : ''}</Notice>}
      <Action secondary loading={listBusy.busy} onClick={() => void listBusy.run(async () => { await loadSubmissions(application.applicationId); })}>读取投递记录</Action>
      {(submissions ?? []).map(record => <Card key={record.submissionId} tone='mint'>
        <Text className='wk-small'>{formatTime(record.occurredAt)} · {CHANNELS.find(option => option.value === record.channel)?.label ?? record.channel} · 确认人 {record.confirmer}</Text>
        {record.boundVersion
          ? <Text className='wk-muted wk-small'>投递版本：V{record.boundVersion.version}（材料 {record.boundVersion.materialId.slice(0, 10)}… · 导出 {record.boundVersion.exportId.slice(0, 10)}… · 摘要 {digestHead(record.boundVersion.contentDigest)}…）</Text>
          : <Text className='wk-muted wk-small'>投递版本：显式未知（不指向任何版本）。</Text>}
        {record.note && <Text className='wk-muted wk-small'>备注：{record.note}</Text>}
      </Card>)}
      {!submissions?.length && submissions !== undefined && <Empty title='还没有投递确认' body='你在外部完成投递后，回到这里记录事实即可。'/>}
    </Card>}

    <Notice tone='info'>结果未知的写入都可以用原请求编号对账或安全重发（同一请求编号服务端不会重复执行）；空间切换后旧响应一律失效。</Notice>
  </Screen>;
}
