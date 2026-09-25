import { useState } from 'react';
import Taro from '@tarojs/taro';
import { Text, View } from '@tarojs/components';
import { Screen, Card, Action, Field, Notice, Badge, Empty, DataBoundary, useData, useAction, useSession } from '../components/ui.tsx';
import * as career from '../services/career.ts';
import type { ApplicationReceipt, MaterialExportReceipt, MaterialView, SubmissionChannel, SubmissionReceipt } from '../../../../packages/api-client/src/career.ts';
import type { EvaluationReceipt } from '../../../../packages/career-core/src/contracts.ts';
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

  const params = Taro.getCurrentInstance().router?.params as Record<string, string | undefined> | undefined;
  const [opportunityId, setOpportunityId] = useState(params?.opportunityId ?? '');
  const [snapshotId, setSnapshotId] = useState(params?.snapshotId ?? '');
  const [evaluation, setEvaluation] = useState<EvaluationReceipt>();
  const [batchIdentity, setBatchIdentity] = useState('');
  const [acknowledged, setAcknowledged] = useState(false);
  const [application, setApplication] = useState<ApplicationReceipt>();
  const [appErrCode, setAppErrCode] = useState<string>();
  const [materialId, setMaterialId] = useState('');
  const [material, setMaterial] = useState<MaterialView>();
  const [sectionHeading, setSectionHeading] = useState('基本情况');
  const [sectionContent, setSectionContent] = useState('');
  const [confirmedVersion, setConfirmedVersion] = useState<number>();
  const [exports, setExports] = useState<MaterialExportReceipt[]>();
  const [checks, setChecks] = useState<ExportOpenRecord[]>([]);
  const [channel, setChannel] = useState<SubmissionChannel>('web');
  const [occurredAt, setOccurredAt] = useState('');
  const [versionChoice, setVersionChoice] = useState('');
  const [note, setNote] = useState('');
  const [submissions, setSubmissions] = useState<SubmissionReceipt[]>();
  const [subErrCode, setSubErrCode] = useState<string>();

  const pendingApplication = career.pendingApplication();
  const pendingMaterial = career.pendingMaterialWrite();
  const pendingSubmission = career.pendingSubmission();
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
    {pendingApplication && <>
      <Notice tone='warning'>有一次结果未知的申请创建（{pendingApplication.requestId.slice(0, 10)}…）。请先用原请求对账，不会重复创建。</Notice>
      <Action secondary loading={recoverAppBusy.busy} onClick={() => void recoverAppBusy.run(async () => { setApplication(await career.reconcilePendingApplication()); })}>用原请求对账申请</Action>
      {recoverAppBusy.error && <Notice tone='danger'>{recoverAppBusy.error} 对账被拒时说明该请求不属于当前空间或不存在；可稍后再试，不会自动重发。</Notice>}
    </>}
    {pendingMaterial && <>
      <Notice tone='warning'>有一次结果未知的材料写入（{pendingMaterial.requestId.slice(0, 10)}…）。</Notice>
      <Action secondary loading={recoverMatBusy.busy} onClick={() => void recoverMatBusy.run(async () => { await career.reconcilePendingMaterial(); setMaterial(await career.material(materialId)); })}>用原请求对账材料</Action>
      {recoverMatBusy.error && <Notice tone='danger'>{recoverMatBusy.error} 对账被拒时说明该请求不属于当前空间或不存在；intent 保留，可稍后再试。</Notice>}
    </>}
    {pendingSubmission && <>
      <Notice tone='warning'>有一次结果未知的投递确认（{pendingSubmission.requestId.slice(0, 10)}…）。请先对账，确认前不会重复记录。</Notice>
      <Action secondary loading={recoverSubBusy.busy} onClick={() => void recoverSubBusy.run(async () => {
        const receipt = await career.reconcilePendingSubmission();
        if (application) await loadSubmissions(application.applicationId);
        void receipt;
      })}>用原请求对账投递</Action>
      {recoverSubBusy.error && <Notice tone='danger'>{recoverSubBusy.error} 对账被拒时说明该请求不属于当前空间或不存在；intent 保留，可稍后再试。</Notice>}
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
    </Card>

    {evaluation && <Card>
      <Text className='wk-h3'>创建求职申请</Text>
      {hardWarning}
      {ineligible && <View className='wk-listrow' onClick={() => setAcknowledged(!acknowledged)}>
        <View className='wk-grow'><Text className={acknowledged ? 'wk-row-title' : 'wk-muted'}>{acknowledged ? '☑' : '☐'} 我已知晓硬性条件不符，仍要显式继续申请</Text></View>
      </View>}
      <Field label='招聘批次标识（同一岗位不同批次可分别申请）' value={batchIdentity} onChange={setBatchIdentity} placeholder='例如：2026 秋招 A 批' />
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='创建求职申请' customStyle={tdesignButtonStyle} loading={applyBusy.busy} onTap={() => void applyBusy.run(async () => {
          try {
            const receipt = await career.createApplication({ opportunityId, snapshotId, evaluationId: evaluation.evaluationId, batchIdentity, continueDespiteHardFailure: ineligible });
            setApplication(receipt);
            setAppErrCode(undefined);
            setMaterialId('');
            setMaterial(undefined);
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
      <Field label='材料编号（创建后自动带入；重进页面可粘贴读取）' value={materialId} onChange={setMaterialId} placeholder='材料编号' />
      <Action secondary loading={matLoadBusy.busy} onClick={() => void matLoadBusy.run(async () => {
        const view = await career.material(materialId);
        setMaterial(view);
        setSectionHeading(view.body.sections[0]?.heading ?? sectionHeading);
        // 同源语义：版本在 Web 或本端确认过即可管理发布/下载/投递绑定。
        if (view.versionCount > 0) setExports((await career.listMaterialExports(materialId)).exports);
      })}>读取材料</Action>
      {matLoadBusy.error && <Notice tone='danger'>{matLoadBusy.error} 若材料不属于当前空间或不存在，会如实提示；不会用演示数据代替。</Notice>}
      {material && <>
        <Text className='wk-muted wk-small'>状态 {material.status} · 已确认版本 {material.versionCount} 个（修改只新增版本，不覆盖旧版）</Text>
        {material.body.sections.map((section, index) => <View key={`${section.heading}-${index}`} className='wk-listrow'>
          <View className='wk-grow'><Text className='wk-row-title'>{section.heading}</Text><Text className='wk-muted wk-small'>{section.content.slice(0, 80)}{section.content.length > 80 ? '…' : ''}</Text></View>
        </View>)}
        {!!material.reviewRisks.length && <Notice tone='warning'>待核风险（如实展示，不虚构）：{material.reviewRisks.map(risk => risk.message).join('；')}</Notice>}
        <Text className='wk-muted wk-small'>版本历史：{material.versions.map(version => `V${version.version}`).join(' · ')}</Text>
      </>}
      <Field label='小节标题' value={sectionHeading} onChange={setSectionHeading} placeholder='例如：教育背景' />
      <Field label='小节内容' value={sectionContent} onChange={setSectionContent} multiline placeholder='这一节要写的内容' />
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='保存材料草稿' customStyle={tdesignButtonStyle} loading={matEditBusy.busy} onTap={() => void matEditBusy.run(async () => {
          const body = { sections: [{ heading: sectionHeading.trim() || '未命名小节', content: sectionContent, claims: [] }] };
          const receipt = material
            ? await career.editMaterial({ materialId, body })
            : await career.editMaterial({ opportunityId: application?.pinnedEvidence.opportunityId ?? opportunityId, snapshotId: application?.pinnedEvidence.snapshotId ?? snapshotId, body });
          setMaterialId(receipt.materialId);
          setSectionContent('');
          setMaterial(await career.material(receipt.materialId));
        })}>保存草稿</t-button>
      </View></View>
      {matEditBusy.error && <Notice tone='danger'>{matEditBusy.error} 冲突时请重读材料后重试。</Notice>}
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='default' ariaLabel='确认材料新版本' customStyle={tdesignButtonStyle} loading={matConfirmBusy.busy} onTap={() => void matConfirmBusy.run(async () => {
          const receipt = await career.confirmMaterial(materialId);
          setConfirmedVersion(receipt.version);
          setMaterial(await career.material(materialId));
        })}>确认为不可变新版本</t-button>
      </View></View>
      {confirmedVersion !== undefined && <Notice tone='info'>已确认版本 V{confirmedVersion}：此后发布与投递都绑定这个确定版本。</Notice>}
      {matConfirmBusy.error && <Notice tone='danger'>{matConfirmBusy.error}</Notice>}
    </Card>}

    {((material?.versionCount ?? 0) > 0 || exports) && <Card>
      <Text className='wk-h3'>发布与下载（PDF / DOCX）</Text>
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='发布双格式导出' customStyle={tdesignButtonStyle} loading={publishBusy.busy} onTap={() => void publishBusy.run(async () => {
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
      <Field label='声明投递时间（留空即记录确认时间）' value={occurredAt} onChange={setOccurredAt} placeholder='例如 2026-09-25 20:00' />
      <Text className='wk-h3'>投递版本</Text>
      {(exports ?? []).filter(exportReceipt => exportReceipt.submittable).map(exportReceipt => <Text key={exportReceipt.exportId} className='wk-small' onClick={() => setVersionChoice(exportReceipt.exportId)}>{versionChoice === exportReceipt.exportId ? '● ' : '○ '}可提交导出 V{exportReceipt.version}（{exportReceipt.exportId.slice(0, 10)}…）</Text>)}
      <Text className='wk-small' onClick={() => setVersionChoice('__unknown__')}>{versionChoice === '__unknown__' ? '● ' : '○ '}版本未知（显式声明，不绑定材料版本）</Text>
      <Field label='备注（可选）' value={note} onChange={setNote} placeholder='例如：官网已投递，附职位链接' />
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='记录投递确认' customStyle={tdesignButtonStyle} loading={submitBusy.busy} onTap={() => void submitBusy.run(async () => {
          try {
            const chosen = (exports ?? []).find(exportReceipt => exportReceipt.exportId === versionChoice);
            const occurred = occurredAt.trim() ? new Date(occurredAt) : undefined;
            await career.recordSubmission({
              applicationId: application.applicationId, channel,
              ...(occurred !== undefined && !Number.isNaN(occurred.getTime()) ? { occurredAt: occurred.toISOString() } : {}),
              ...(chosen ? { materialId: chosen.materialId, exportId: chosen.exportId } : {}),
              versionUnknown: !chosen,
              ...(note.trim() ? { note } : {}),
            });
            setSubErrCode(undefined);
            await loadSubmissions(application.applicationId);
          } catch (error) { setSubErrCode(typedCode(error)); throw error; }
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
