import { useRef, useState } from 'react';
import Taro from '@tarojs/taro';
import { Text, View } from '@tarojs/components';
import { Screen, Card, Action, Notice, Badge, DataBoundary, useData, useAction, useSession, confirmAction } from '../components/ui.tsx';
import * as career from '../services/career.ts';
import { spaceExportPayload, saveSpaceExportPackage, copySpaceExportToClipboard, type SpaceExportSaveRecord } from '../adapters/career-platform.ts';
import { lifecycleGating, createDeletionPageController, confirmAbandonIntent } from './export-deletion.gating.ts';
import type { CareerExportReceipt, CareerDeletionBoundaryView, CareerDeletionReceipt } from '../../../../packages/api-client/src/career.ts';
import { formatTime, formatBytes } from '../core/format.ts';
import { logout } from '../services/runtime.ts';
import { navigate } from '../platform/navigation.ts';

// T32 全空间导出与完整删除：与 Web ExportDeletionPage 同源同版本（同一批 api-client
// 解码器，同一冻结合同）。导出=同步内联归档（六段包含性清单）+ 本机留存（平台无浏览器式
// 下载，等效可取得流程=USER_DATA_PATH 文件+完整内容剪贴板，不静默截断）；删除先呈现
// 保留规则与外部平台边界清单→强制确认→部分失败保留可恢复状态（绝不称已完全删除）→
// 删除完成后旧导出授权复验（404）+ 本机 wk:career:* 缓存清理 + 退出重进不恢复。
const stepNameLabels: Record<string, string> = {
  revoke_material_exports: '撤销材料导出与下载授权',
  purge_career_data: '清除空间内求职数据',
  remove_workbench_tasks: '移除 Workbench 申请任务投影',
  finalize: '落定删除',
};
const stepStatusLabels: Record<string, string> = { pending: '待执行', done: '已完成', failed: '失败' };
const deletionStatusLabels: Record<string, string> = { deleting: '删除进行中', partial: '部分失败（未完全删除，可恢复）', deleted: '已完全删除' };
const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
const digestHead = (digest: string): string => digest.slice(0, 12);
const typedCode = (error: unknown): string | undefined => { const code = (error as { code?: unknown } | null | undefined)?.code; return typeof code === 'string' ? code : undefined; };

export default function ExportDeletionPage() {
  const session = useSession();
  const desk = useData(`career:${session.userId}:${session.tenantId}`, () => career.loadCareer());
  const exportBusy = useAction(); const saveBusy = useAction(); const copyBusy = useAction();
  const boundaryBusy = useAction(); const deletionBusy = useAction();
  const recExportBusy = useAction(); const retryExportBusy = useAction();
  const recDelBusy = useAction(); const retryDelBusy = useAction(); const logoutBusy = useAction();

  const [exported, setExported] = useState<CareerExportReceipt>();
  const [saved, setSaved] = useState<SpaceExportSaveRecord>();
  const [copyNotice, setCopyNotice] = useState('');
  const [expErrCode, setExpErrCode] = useState<string>();
  const [boundary, setBoundary] = useState<CareerDeletionBoundaryView>();
  const [acknowledged, setAcknowledged] = useState(false);
  const [deletion, setDeletion] = useState<CareerDeletionReceipt>();
  const [delErrCode, setDelErrCode] = useState<string>();
  const [verifyNotice, setVerifyNotice] = useState('');
  const [recoveryNotice, setRecoveryNotice] = useState('');
  const [exportRecoveryConfirmationBusy, setExportRecoveryConfirmationBusy] = useState(false);
  const [deletionRecoveryConfirmationBusy, setDeletionRecoveryConfirmationBusy] = useState(false);
  const [clearedKeys, setClearedKeys] = useState<string[]>();
  // 修复轮 2 F1：partial 确定回执后的恢复尝试（对账/重试）以未决告终时，旧 partial 回执
  // 不再代表当前结果——置位后删除主按钮与知悉回到 unknown 封锁，防止换新 requestId 重复
  // 发起删除（Web runDeletion/lookupDeletionReceipt 失败 → phase='unknown'）。
  const [, setDelRecoveryUnresolved] = useState(false);
  const deletionController = useRef(createDeletionPageController()).current;
  // 终态化处理只做一次（同一删除 requestId 的收尾不重复执行）。
  const finalizedFor = useRef<string | undefined>(undefined);
  const exportAbandonInFlight = useRef(false);
  const deletionAbandonInFlight = useRef(false);
  const exportOperationInFlight = useRef(false);
  const deletionOperationInFlight = useRef(false);

  // 恢复态读取自受控存储：每次渲染重读（useAction 状态变化触发重渲染）。
  const pendingExport = career.pendingSpaceExport();
  const pendingDeletion = career.pendingSpaceDeletion();
  const revision = desk.data?.revision;
  const deleted = deletion?.status === 'deleted';
  const exportRecoveryBusy = exportBusy.busy || recExportBusy.busy || retryExportBusy.busy;
  const deletionRecoveryBusy = deletionBusy.busy || recDelBusy.busy || retryDelBusy.busy;

  // 修复轮 M1：主按钮门控对齐 Web ExportDeletionPage——结果未知（intent 存续）与
  // 发起/对账/重试进行中都封锁主操作（防重复发起与竞态）；导出未决联动封锁删除与
  // 知悉勾选；partial 是已呈报的确定回执（Web phase=error），不按 unknown 封锁。
  const gating = lifecycleGating({
    exportBusy: exportBusy.busy || recExportBusy.busy || retryExportBusy.busy,
    exportUnknown: pendingExport !== null,
    deletionBusy: deletionBusy.busy || recDelBusy.busy || retryDelBusy.busy,
    deletionUnknown: deletionController.isUnknown(pendingDeletion !== null),
    revisionLoaded: revision !== undefined,
    boundaryShown: boundary !== undefined,
    acknowledged,
    deleted,
  });

  const acceptExport = (receipt: CareerExportReceipt): void => {
    setExported(receipt);
    setSaved(undefined);
    setCopyNotice('');
  };

  // 删除终态收尾：本机缓存清理（storage 中 wk:career:* + 共享 desk）→ 旧导出授权复验。
  // 两者都如实呈现结果；读得到旧回执是验证失败，绝不静默通过。
  const finalizeAfterDeletion = async (receipt: CareerDeletionReceipt): Promise<void> => {
    if (finalizedFor.current === receipt.requestId) return;
    finalizedFor.current = receipt.requestId;
    const cleared = career.clearCareerCaches();
    setClearedKeys(cleared);
    desk.reload();
    if (!exported) {
      setVerifyNotice('删除后验证：本次会话未发起过导出，未执行旧授权复验（如需该验证，先导出再删除）。');
      return;
    }
    const original = exported.requestId;
    try {
      await career.spaceExportReceipt(original);
      setVerifyNotice(`删除后验证异常：导出回执 ${original.slice(0, 10)}… 仍可读取，旧授权可能未失效。请退出本页重新进入核对删除结果。`);
    } catch (error) {
      setVerifyNotice(career.isReceiptMissing(error)
        ? `删除后验证：导出回执 ${original.slice(0, 10)}… 已不可读取（旧授权与旧入口已失效）。`
        : `删除后验证暂时无法完成：${(error as Error).message}`);
    }
  };

  const acceptDeletion = (receipt: CareerDeletionReceipt): void => {
    deletionController.accept(receipt.status);
    setDeletion(receipt);
    setAcknowledged(false);
    setDelRecoveryUnresolved(false); // 确定回执落定上次恢复尝试的未决（Web acceptDeletion）
    if (receipt.status === 'deleted') void finalizeAfterDeletion(receipt);
  };

  // 恢复入口统一包装（修复轮 2 F1）：失败按 Web 语义置/清未决——对账失败（intent 仍在）
  // 一律未决，重试仅 ambiguous 未决、definite 拒绝解除封锁；成功拿到确定回执即落定。
  const reconcileDeletion = (): void => {
    if (deletionOperationInFlight.current) return;
    deletionOperationInFlight.current = true;
    void recDelBusy.run(async () => {
      try { acceptDeletion(await career.reconcilePendingSpaceDeletion()); }
      catch (error) { deletionController.recoveryFailed('reconcile', error, career.pendingSpaceDeletion() !== null); setDelRecoveryUnresolved(deletionController.isRecoveryUnresolved()); throw error; }
      finally { deletionOperationInFlight.current = false; }
    });
  };
  const retryDeletion = (): void => {
    if (deletionOperationInFlight.current) return;
    deletionOperationInFlight.current = true;
    void retryDelBusy.run(async () => {
      try { acceptDeletion(await career.retryPendingSpaceDeletion()); }
      catch (error) { deletionController.recoveryFailed('retry', error, career.pendingSpaceDeletion() !== null); setDelRecoveryUnresolved(deletionController.isRecoveryUnresolved()); throw error; }
      finally { deletionOperationInFlight.current = false; }
    });
  };

  const abandonExportRecovery = async (): Promise<void> => {
    const expectedRequestId = pendingExport?.requestId;
    if (!expectedRequestId) {
      setRecoveryNotice('未放弃导出恢复：当前显示的恢复请求已变化或不存在，本机记录未清除。');
      return;
    }
    if (exportAbandonInFlight.current) return;
    exportAbandonInFlight.current = true;
    setExportRecoveryConfirmationBusy(true);
    try {
      const result = await confirmAbandonIntent(expectedRequestId, {
        confirm: () => confirmAction('放弃导出恢复？', '这只会清除本机恢复记录；原导出请求可能已经在服务端生效。清除后本机不再保留原请求编号。'),
        currentRequestId: () => career.pendingSpaceExport()?.requestId,
        isBusy: () => exportOperationInFlight.current,
        abandon: id => career.abandonPendingSpaceExport(id),
      });
      setRecoveryNotice(result === 'abandoned'
        ? '已放弃导出恢复：仅清除了本机恢复记录；原导出请求可能已在服务端生效。'
        : result === 'cancelled'
          ? '未放弃导出恢复：已取消确认，本机恢复记录仍保留。'
          : result === 'busy'
            ? '未放弃导出恢复：导出或恢复操作正在进行，本机恢复记录仍保留。'
            : '未放弃导出恢复：当前请求编号已变化，本机恢复记录未清除。');
    } catch {
      setRecoveryNotice('未放弃导出恢复：确认未完成，本机恢复记录仍保留。');
    } finally {
      exportAbandonInFlight.current = false;
      setExportRecoveryConfirmationBusy(false);
    }
  };

  const abandonDeletionRecovery = async (): Promise<void> => {
    const expectedRequestId = pendingDeletion?.requestId;
    if (!expectedRequestId) {
      setRecoveryNotice('未放弃删除恢复：当前显示的恢复请求已变化或不存在，本机记录未清除。');
      return;
    }
    if (deletionAbandonInFlight.current) return;
    deletionAbandonInFlight.current = true;
    setDeletionRecoveryConfirmationBusy(true);
    try {
      const result = await confirmAbandonIntent(expectedRequestId, {
        confirm: () => confirmAction('放弃删除恢复？', '这只会清除本机恢复记录；原删除操作可能已经在服务端生效。清除后本机不再保留原请求编号。'),
        currentRequestId: () => career.pendingSpaceDeletion()?.requestId,
        isBusy: () => deletionOperationInFlight.current || career.spaceDeletionRecoveryActive(expectedRequestId),
        abandon: id => career.abandonPendingSpaceDeletion(id),
      });
      if (result === 'abandoned') {
        deletionController.clear();
        setDeletion(undefined);
        setDelRecoveryUnresolved(false);
        setAcknowledged(false);
        setRecoveryNotice('已放弃删除恢复：仅清除了本机恢复记录；原删除操作可能已在服务端生效。');
      } else {
        setRecoveryNotice(result === 'cancelled'
          ? '未放弃删除恢复：已取消确认，本机恢复记录仍保留。'
          : result === 'busy'
            ? '未放弃删除恢复：删除或恢复操作正在进行，本机恢复记录仍保留。'
            : '未放弃删除恢复：当前请求编号已变化，本机恢复记录未清除。');
      }
    } catch {
      setRecoveryNotice('未放弃删除恢复：确认未完成，本机恢复记录仍保留。');
    } finally {
      deletionAbandonInFlight.current = false;
      setDeletionRecoveryConfirmationBusy(false);
    }
  };

  const snapshotTotal = exported ? exported.archive.opportunities.reduce((total, item) => total + item.snapshots.length, 0) : 0;
  const eventTotal = exported ? exported.archive.applications.reduce((total, item) => total + item.progressEvents.length, 0) : 0;
  const versionTotal = exported ? exported.archive.materials.reduce((total, item) => total + item.versions.length, 0) : 0;

  return <Screen title='导出与删除'>
    <Text className='wk-display'>一份完整导出，{'\n'}一次不可逆删除。</Text>
    <Notice tone='info'>导出生成完整导出包（档案、原始岗位快照、申请事件、材料版本、投递记录），内容与 Web 端同一后端包。完整删除不可恢复：外部平台的投递与已发出的副本不受本系统控制、无法撤回；删除前会先呈现边界清单，部分失败保留可恢复状态，绝不声称已完全删除。</Notice>

    {/* 恢复态置顶（T24/T26 教训）：结果未知的写入第一屏可见、可操作。 */}
    {pendingExport && <>
      <Notice tone='warning'>有一次结果未知的导出（{pendingExport.requestId.slice(0, 10)}…）。请先用原请求对账，不会重复执行。</Notice>
      <Action secondary loading={recExportBusy.busy} onClick={() => {
        if (exportOperationInFlight.current) return;
        exportOperationInFlight.current = true;
        void recExportBusy.run(async () => { try { acceptExport(await career.reconcilePendingSpaceExport()); } finally { exportOperationInFlight.current = false; } });
      }}>用原请求对账导出</Action>
      {recExportBusy.error && <Notice tone='danger'>{recExportBusy.error} 对账被拒时说明该请求不存在或不属于当前空间；可再用原编号重试。</Notice>}
      <Action secondary loading={retryExportBusy.busy} onClick={() => {
        if (exportOperationInFlight.current) return;
        exportOperationInFlight.current = true;
        void retryExportBusy.run(async () => { try { acceptExport(await career.retryPendingSpaceExport()); } finally { exportOperationInFlight.current = false; } });
      }}>用原请求编号重试导出</Action>
      {retryExportBusy.error && <Notice tone='danger'>{retryExportBusy.error} 重试沿用原请求编号，服务端幂等不会重复执行。</Notice>}
      <Action secondary disabled={exportRecoveryBusy || exportRecoveryConfirmationBusy} onClick={() => void abandonExportRecovery()}>放弃导出恢复</Action>
    </>}
    {pendingDeletion && !deleted && <>
      <Notice tone='warning'>有一次未到终态的删除（{pendingDeletion.requestId.slice(0, 10)}…）。可用原请求对账最新状态，或用原编号重试恢复；不会发起新的删除。</Notice>
      <Action secondary loading={recDelBusy.busy} onClick={reconcileDeletion}>用原请求对账删除</Action>
      {recDelBusy.error && <Notice tone='danger'>{recDelBusy.error} 对账被拒时说明该请求不存在或不属于当前空间；intent 保留，可稍后再试。</Notice>}
      <Action secondary danger loading={retryDelBusy.busy} onClick={retryDeletion}>用原请求编号重试删除</Action>
      {retryDelBusy.error && <Notice tone='danger'>{retryDelBusy.error} 重试沿用原请求编号；服务端按步骤续跑，已完成步骤不会重复执行。</Notice>}
      <Action secondary disabled={deletionRecoveryBusy || deletionRecoveryConfirmationBusy} onClick={() => void abandonDeletionRecovery()}>放弃删除恢复</Action>
    </>}
    {recoveryNotice && <Notice tone='info'>{recoveryNotice}</Notice>}

    <DataBoundary state={desk}>{view => view && <Card>
      <Text className='wk-muted wk-small'>当前档案修订 {view.revision}（导出与删除将按此修订提交）</Text>
      <Text className='wk-muted wk-small'>已确认事实 {view.facts.length} 条 · 待处理提案 {view.proposals.length} 条</Text>
    </Card>}</DataBoundary>

    <Card>
      <Text className='wk-h3'>导出数据</Text>
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='发起导出' customStyle={tdesignButtonStyle} loading={exportBusy.busy} disabled={gating.exportDisabled} onTap={() => {
          if (exportOperationInFlight.current) return;
          exportOperationInFlight.current = true;
          void exportBusy.run(async () => {
            try {
              if (gating.exportDisabled) return; // 防重入（Web runExport busy/unknown 守卫）
              try {
                acceptExport(await career.exportWholeSpace());
                setExpErrCode(undefined);
              } catch (error) { setExpErrCode(typedCode(error)); throw error; }
            } finally { exportOperationInFlight.current = false; }
          });
        }}>发起导出（只读，不改变档案）</t-button>
      </View></View>
      {exportBusy.error && <Notice tone='danger'>{exportBusy.error}{expErrCode === 'revision_conflict' ? ' 档案已更新：重新读取修订后再导出（新导出将使用新请求编号）。' : ''}</Notice>}
      {expErrCode === 'revision_conflict' && <Action secondary onClick={() => desk.reload()}>重新读取档案修订</Action>}
      {exported && <Card tone='mint'>
        <Text className='wk-h3'>导出包（导出编号 {exported.exportId.slice(0, 12)}…）</Text>
        <Text className='wk-muted wk-small'>摘要 {digestHead(exported.digest)}… · 固定档案修订 {exported.revision} · 生成于 {formatTime(exported.createdAt)} · 状态：完成</Text>
        <Text className='wk-small'>包含性清单（与 Web 端同一后端包，逐段如实计数）：</Text>
        <Text className='wk-muted wk-small'>档案事实：{exported.archive.profile.facts.length} 条（待处理提案 {exported.archive.profile.proposals.length} 条）</Text>
        <Text className='wk-muted wk-small'>事实历史：{exported.archive.factHistory.length} 条</Text>
        <Text className='wk-muted wk-small'>岗位与原始快照：{exported.archive.opportunities.length} 个岗位 / {snapshotTotal} 份快照</Text>
        <Text className='wk-muted wk-small'>申请与进展事件：{exported.archive.applications.length} 个申请 / {eventTotal} 条事件</Text>
        <Text className='wk-muted wk-small'>材料与版本：{exported.archive.materials.length} 份材料 / {versionTotal} 个版本</Text>
        <Text className='wk-muted wk-small'>投递记录：{exported.archive.submissions.length} 条</Text>
        <Action secondary loading={saveBusy.busy} onClick={() => void saveBusy.run(async () => {
          const payload = spaceExportPayload(exported);
          setSaved(await saveSpaceExportPackage(payload, exported.exportId));
        })}>保存导出包到本机</Action>
        {saved && <Notice tone='info'>已保存完整导出包：{saved.filePath}（{formatBytes(saved.bytes)}，本地副本摘要 {digestHead(saved.digest)}… 由落盘字节复算）。微信小程序无法像浏览器那样写入手机任意目录：该文件保存在小程序的本机持久存储中；需要长期留存，请再复制完整内容并转移到你的留存位置。此文件是你的留存副本，之后执行删除不会移动它。</Notice>}
        {saveBusy.error && <Notice tone='danger'>{saveBusy.error}</Notice>}
        <Action secondary loading={copyBusy.busy} onClick={() => void copyBusy.run(async () => {
          await copySpaceExportToClipboard(spaceExportPayload(exported));
          setCopyNotice('完整导出内容（未截断）已复制到剪贴板，可粘贴到任何位置留存。');
        })}>复制完整导出内容</Action>
        {copyNotice && <Notice tone='info'>{copyNotice}</Notice>}
        {copyBusy.error && <Notice tone='danger'>{copyBusy.error}</Notice>}
      </Card>}
    </Card>

    <Card>
      <Text className='wk-h3'>完整删除</Text>
      <Action secondary loading={boundaryBusy.busy} onClick={() => void boundaryBusy.run(async () => { setBoundary(await career.deletionBoundary()); })}>{boundary ? '刷新删除边界' : '查看删除边界'}</Action>
      {boundaryBusy.error && <Notice tone='danger'>{boundaryBusy.error} 未呈现边界清单前不能发起删除；可稍后重试。</Notice>}
      {boundary && <Card tone='warning'>
        <Text className='wk-small'>空间内将删除的数据</Text>
        {boundary.inSpace.map(section => <Text key={section.section} className='wk-muted wk-small'>{section.section}（{section.description}）：{section.count} 项</Text>)}
        <Text className='wk-small'>外部平台资料（本系统不可控）</Text>
        {boundary.external.map(item => <Text key={item.item} className='wk-muted wk-small'>{item.description}{item.revocable ? '' : '（不可撤回）'}</Text>)}
        <Text className='wk-small'>保留范围与状态</Text>
        {boundary.retention.map(item => <Text key={item.holder} className='wk-muted wk-small'>{item.holder}：{item.reason}（{item.status}）</Text>)}
        <View className='wk-listrow' onClick={() => { if (!gating.acknowledgeDisabled) setAcknowledged(!acknowledged); }}>
          <View className='wk-grow'><Text className={acknowledged ? 'wk-row-title' : 'wk-muted'}>{acknowledged ? '☑' : '☐'} 我已知悉外部平台资料不可撤回、保留范围如上，并理解完整删除不可恢复。</Text></View>
        </View>
      </Card>}
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='发起完整删除' customStyle={tdesignButtonStyle} loading={deletionBusy.busy} disabled={gating.deletionDisabled} onTap={() => {
          if (deletionOperationInFlight.current) return;
          deletionOperationInFlight.current = true;
          void deletionBusy.run(async () => {
            try {
              if (gating.deletionDisabled) return; // 防重入（Web runDeletion busy/unknown 守卫）
              const confirmed = await confirmAction('确认完整删除？', '空间内求职数据将被删除且不可恢复；外部平台的投递与已发出的副本不受本系统控制。');
              if (!confirmed) return;
              try {
                acceptDeletion(await career.deleteWholeSpace());
                setDelErrCode(undefined);
              } catch (error) {
                setDelErrCode(typedCode(error));
                if (typedCode(error) === 'outcome_unknown') { deletionController.initialOutcomeUnknown(); setDelRecoveryUnresolved(true); }
                throw error;
              }
            } finally { deletionOperationInFlight.current = false; }
          });
        }}>发起完整删除</t-button>
      </View></View>
      {deletionBusy.error && <Notice tone='danger'>{deletionBusy.error}{delErrCode === 'revision_conflict' ? ' 档案已更新：重新读取修订、确认边界后再次发起（新删除会使用新的请求编号）。' : delErrCode === 'idempotency_conflict' ? ' 请求编号已对应其他内容，服务器拒绝了本次删除。可回到恢复入口用原编号对账。' : delErrCode === 'unresolved_action' ? ' 尚有删除恢复记录：请使用原请求对账/重试，或先在恢复区确认放弃，再开始新的完整删除。' : ''}</Notice>}
      {delErrCode === 'revision_conflict' && <Action secondary onClick={() => desk.reload()}>重新读取档案修订</Action>}
      {!boundary && !boundaryBusy.error && <Notice tone='info'>先查看删除边界清单并勾选知悉，才能发起删除。</Notice>}
    </Card>

    {deletion && <Card tone={deletion.status === 'deleted' ? 'mint' : deletion.status === 'partial' ? 'warning' : 'white'}>
      <View className='wk-between'>
        <Badge tone={deleted ? 'success' : deletion.status === 'partial' ? 'danger' : 'warning'}>{deletionStatusLabels[deletion.status]}</Badge>
        <Text className='wk-muted wk-small'>请求 {deletion.requestId.slice(0, 10)}…</Text>
      </View>
      <Text className='wk-small'>起始于 {formatTime(deletion.startedAt)}{deletion.completedAt ? ` · 完成于 ${formatTime(deletion.completedAt)}` : ''} · 空间修订 {deletion.revision}</Text>
      {deletion.steps.map(step => <View key={step.name} className='wk-listrow'>
        <View className='wk-grow'>
          <Text className='wk-row-title'>{stepNameLabels[step.name] ?? step.name}</Text>
          {step.status === 'failed' && step.detail ? <Text className='wk-muted wk-small'>{step.detail}</Text> : null}
        </View>
        <Badge tone={step.status === 'done' ? 'success' : step.status === 'failed' ? 'danger' : 'neutral'}>{stepStatusLabels[step.status] ?? step.status}</Badge>
      </View>)}
      <Text className='wk-h3'>保留范围与状态</Text>
      {deletion.retention.map(item => <Text key={item.holder} className='wk-muted wk-small'>{item.holder}：{item.reason}（{item.status}）</Text>)}
      {deletion.status === 'partial' && <>
        <Notice tone='warning'>删除部分失败：未完全删除。失败步骤已列出，状态与审计已保留，可用原请求编号重试恢复；本页不会显示"已完全删除"。</Notice>
        <Action secondary danger loading={retryDelBusy.busy} onClick={retryDeletion}>用原请求编号重试删除</Action>
      </>}
      {deleted && <>
        <Notice tone='info'>空间已完全删除（完成于 {formatTime(deletion.completedAt)}）。保留范围已在上方披露。服务端数据已删除；重新登录同一微信账号不会恢复旧资料。</Notice>
        {verifyNotice && <Notice tone='info'>{verifyNotice}</Notice>}
        {clearedKeys && <Notice tone='info'>本机求职缓存已清理：{clearedKeys.length} 个 wk:career:* 存储键已移除（登录凭据与其他数据不动）。</Notice>}
        <Action danger loading={logoutBusy.busy} onClick={() => void logoutBusy.run(async () => { await logout(); })}>退出登录（重新登录后复验旧资料不恢复）</Action>
        {logoutBusy.error && <Notice tone='danger'>{logoutBusy.error}</Notice>}
        <Action secondary onClick={() => void navigate('career')}>回到求职工作台</Action>
      </>}
    </Card>}

    <Notice tone='info'>结果未知的写入都可以用原请求编号对账或安全重发（同一请求编号服务端不会重复执行）；空间切换后旧响应一律失效。</Notice>
  </Screen>;
}
