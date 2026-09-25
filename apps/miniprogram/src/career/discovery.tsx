import { useState } from 'react';
import Taro, { useShareAppMessage } from '@tarojs/taro';
import { Text, View } from '@tarojs/components';
import { Screen, Card, Action, Field, Notice, Badge, Empty, DataBoundary, useData, useAction, useSession } from '../components/ui.tsx';
import { navigate } from '../platform/navigation.ts';
import { auth } from '../services/runtime.ts';
import * as career from '../services/career.ts';
import { careerPlatform, type SharedImportDraft } from '../adapters/career-platform.ts';
import { formatTime } from '../core/format.ts';
import { errorMessage } from '../core/errors.ts';

// T24 求职工作台：C 找岗（searches 一次性合同）、建档（上传简历/逐步建档 + 逐项确认）、
// 分享导入（先核对后提交）。微信身份只关联既有 WeKnora 档案，本端没有任何创建档案调用。
const FIELDS = ['毕业时间', '学历', '城市', '意向'] as const;
const qualificationLabel: Record<string, string> = { qualified: '符合', not_qualified: '不符合', needs_review: '待核对' };
const qualificationTone: Record<string, 'success' | 'danger' | 'warning'> = { qualified: 'success', not_qualified: 'danger', needs_review: 'warning' };
const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';

export default function DiscoveryPage() {
  const session = useSession();
  const query = useData(`career:${session.userId}:${session.tenantId}`, () => career.loadCareer());
  const confirmBusy = useAction(); const dismissBusy = useAction(); const proposeBusy = useAction(); const uploadBusy = useAction();
  const searchBusy = useAction(); const recoverBusy = useAction(); const importBusy = useAction();
  const reconcileBusy = useAction(); const resendBusy = useAction();
  const [searchInput, setSearchInput] = useState('');
  const [searchOut, setSearchOut] = useState<career.SearchOutcome>();
  const [receiptMissing, setReceiptMissing] = useState(false);
  const [factKey, setFactKey] = useState<string>(FIELDS[0]);
  const [factValue, setFactValue] = useState('');
  const [entry] = useState(() => careerPlatform.readSharedEntry());
  const [draft, setDraft] = useState<SharedImportDraft | undefined>(() => (entry ? career.prepareSharedImport(entry.text, entry.sourceLabel) : undefined));
  const [pasteText, setPasteText] = useState('');
  const [importNotice, setImportNotice] = useState('');
  // T26：导入成功后保住岗位/快照编号，作为进入“申请与材料”的入口参数。
  const [imported, setImported] = useState<{ opportunityId: string; snapshotId: string }>();
  // 分享导出：把当前核对中的 JD 原文随卡片带给接收方（先核对后提交的另一半）。
  useShareAppMessage(() => ({ title: draft ? `职位核对：${draft.preview.excerpt.slice(0, 20)}` : 'WeKnora 求职工作台', path: `/career/discovery${draft ? `?jd=${encodeURIComponent(draft.rawText)}` : ''}` }));

  const pendingFactAction = career.pendingAction();
  const pendingSearch = career.pendingSearch();
  // 额度不足专属提示（typed 429 search_quota_refused）经 errorMessage 真实可达：
  // useAction 存的串即页面呈现文案，专属句式同时决定 Notice 降为 warning 而非失败。
  const quotaRefused = (message?: string) => message?.includes('搜索额度不足') ?? false;

  const copyLink = (link: string) => { void Taro.setClipboardData({ data: link }).then(() => Taro.showToast({ title: '链接已复制', icon: 'none' })); };

  return <Screen title='求职工作台'>
    <Text className='wk-display'>同一个账号，{'\n'}同一份求职档案。</Text>
    <Notice tone='info'>微信端只关联你已有的平台账号与档案；Web 端的修改在这里刷新即可见。</Notice>

    {pendingFactAction && <Notice tone='warning'>有结果未知的档案操作（{pendingFactAction.requestId.slice(0, 10)}…）。</Notice>}
    {pendingFactAction && <Action secondary loading={recoverBusy.busy} onClick={() => void recoverBusy.run(async () => { await career.reconcilePending(); query.reload(); })}>用原请求对账恢复</Action>}

    <DataBoundary state={query}>{view => view && <>
      {career.needsOnboarding(view) && <Empty title='还没有求职档案' body='上传已有简历或逐项建档；每条抽取的事实都需要你逐项确认后才生效。'/>}
      {!!view.facts.length && <Card>{view.facts.map(fact => <View key={fact.key} className='wk-listrow'><View className='wk-grow'><Text className='wk-row-title'>{fact.key}：{fact.value}</Text><Text className='wk-muted wk-small'>已确认 · 修订 {fact.revision} · {formatTime(fact.confirmedAt)}</Text></View><Badge tone='success'>已确认</Badge></View>)}</Card>}

      {!!view.proposals.length && <Card tone='warning'>
        <Text className='wk-h3'>待你逐项确认的抽取事实</Text>
        {view.proposals.map(proposal => <View key={proposal.id}>
          <View className='wk-listrow'>
            <View className='wk-grow'><Text className='wk-row-title'>{proposal.key}：{proposal.value}</Text><Text className='wk-muted wk-small'>来源 {proposal.source.kind} · 未确认不生效</Text></View>
            <Badge tone='warning'>待确认</Badge>
          </View>
          <View className='wk-between'>
            <View className='wk-tdesign-scope'>
              {/* D2：t-button 必须以 JSX 书写，构建期才会收集属性与 tap 事件绑定。 */}
              <t-button size='medium' theme='primary' ariaLabel={`确认 ${proposal.key}`} customStyle={tdesignButtonStyle} loading={confirmBusy.busy} onTap={() => void confirmBusy.run(async () => { await career.confirmProposal(proposal.id); query.reload(); })}>确认生效</t-button>
            </View>
            <View className='wk-tdesign-scope'>
              <t-button size='medium' theme='default' ariaLabel={`放弃 ${proposal.key}`} customStyle={tdesignButtonStyle} loading={dismissBusy.busy} onTap={() => void dismissBusy.run(async () => { await career.dismissProposal(proposal.id); query.reload(); })}>放弃</t-button>
            </View>
          </View>
        </View>)}
      </Card>}

      <Card>
        <Text className='wk-h3'>建档</Text>
        <View className='wk-between'><View className='wk-tdesign-scope'>
          <t-button block size='large' theme='primary' ariaLabel='上传已有简历' customStyle={tdesignButtonStyle} loading={uploadBusy.busy} onTap={() => void uploadBusy.run(async () => {
            const stamp = auth.scope.capture();
            const file = await careerPlatform.chooseResume();
            if (!auth.scope.isCurrent(stamp)) throw new Error('SCOPE_CHANGED');
            const upload = await career.uploadResume(file);
            setImportNotice(upload.receipt ? `简历已上传：抽取到 ${upload.receipt.proposals.length} 条待确认事实，请逐项确认。` : '简历已上传，解析结果稍后可在待确认列表出现。');
            query.reload();
          })}>上传已有简历</t-button>
        </View></View>
        {uploadBusy.error && <Notice tone='danger'>{uploadBusy.error}</Notice>}
        <Text className='wk-muted wk-small'>或逐步建档：每填一项都会先成为待确认事实。</Text>
        {FIELDS.map(field => <Text key={field} className='wk-small' onClick={() => setFactKey(field)}>{factKey === field ? '● ' : '○ '}{field}</Text>)}
        <Field label={`补充：${factKey}`} value={factValue} onChange={setFactValue} placeholder='例如：本科' />
        <Action secondary disabled={proposeBusy.busy} loading={proposeBusy.busy} onClick={() => void proposeBusy.run(async () => {
          await career.proposeFact(factKey, factValue.trim()); setFactValue(''); query.reload();
        })}>添加为待确认事实</Action>
        {proposeBusy.error && <Notice tone='danger'>{proposeBusy.error}</Notice>}
        {(confirmBusy.error || dismissBusy.error) && <Notice tone='danger'>{`${confirmBusy.error ?? dismissBusy.error} 冲突时下拉重读最新档案后再试。`}</Notice>}
      </Card>
    </>}</DataBoundary>

    <Card>
      <Text className='wk-h3'>找岗（一次性搜索）</Text>
      <Field label='想找什么' value={searchInput} onChange={setSearchInput} placeholder='岗位、方向或要求' />
      <View className='wk-between'><View className='wk-tdesign-scope'>
        <t-button block size='large' theme='primary' ariaLabel='发起一次性搜索' customStyle={tdesignButtonStyle} loading={searchBusy.busy} onTap={() => void searchBusy.run(async () => { setSearchOut(await career.searchOnce(searchInput)); })}>搜索</t-button>
      </View></View>
      {searchBusy.error && <Notice tone={quotaRefused(searchBusy.error) ? 'warning' : 'danger'}>{searchBusy.error}</Notice>}
      {pendingSearch && <Notice tone='warning'>有一次结果未知的搜索（{pendingSearch.query}）。</Notice>}
      {pendingSearch && <Action secondary loading={reconcileBusy.busy} onClick={() => void reconcileBusy.run(async () => {
        setReceiptMissing(false);
        try {
          setSearchOut(await career.searchReceipt());
        } catch (error) {
          // 对账确认服务端无回执（404）：不是失败终态，而是可操作恢复入口。
          if (career.isReceiptMissing(error)) { setReceiptMissing(true); return; }
          throw error;
        }
      })}>用原请求对账</Action>}
      {reconcileBusy.error && <Notice tone='danger'>{reconcileBusy.error}</Notice>}
      {receiptMissing && <>
        <Notice tone='warning'>尚未找到该请求的回执：搜索请求可能未送达服务器。可安全重发（同一请求 ID，服务端不会重复执行），或稍后再用原请求对账。</Notice>
        <Action secondary loading={resendBusy.busy} onClick={() => void resendBusy.run(async () => { setSearchOut(await career.retryPendingSearch()); setReceiptMissing(false); })}>安全重发原搜索</Action>
      </>}
      {resendBusy.error && <Notice tone={quotaRefused(resendBusy.error) ? 'warning' : 'danger'}>{resendBusy.error}</Notice>}
      {searchOut && <>
        {searchOut.status === 'failed' && <Notice tone='danger'>搜索未完成（{searchOut.failureCode ?? '未知原因'}）。</Notice>}
        {searchOut.hasNoVettedSources && <Notice tone='warning'>当前没有已核验的搜索来源，本次未返回任何结果行；来源接入后会如实列出。</Notice>}
        {!!searchOut.scopeNotes.length && <Notice tone='info'>{searchOut.scopeNotes.join('；')}</Notice>}
        <Text className='wk-muted wk-small'>覆盖来源（{searchOut.sources.length}）：{searchOut.sources.map(source => `${source.label}${source.available ? '' : `（不可用${source.failureCode ? `：${source.failureCode}` : ''}）`}`).join('、') || '无'}</Text>
        {searchOut.rows.map(row => <Card key={row.resultId} tone='mint'>
          <View className='wk-between'><Badge tone={qualificationTone[row.qualification] ?? 'neutral'}>{qualificationLabel[row.qualification] ?? row.qualification}</Badge><Text className='wk-muted wk-small'>检查于 {formatTime(row.checkedAt)}</Text></View>
          <Text className='wk-small'>来源 {row.sourceId}{row.uncertainty ? ` · 不确定性：${row.uncertainty === 'low_confidence' ? '低置信' : row.uncertainty}` : ''}</Text>
          <Text className='wk-row-title' onClick={() => copyLink(row.link)}>{row.link}</Text>
          <Action secondary onClick={() => copyLink(row.link)}>复制原始链接</Action>
        </Card>)}
      </>}
    </Card>

    <Card>
      <Text className='wk-h3'>分享导入</Text>
      {draft && <>
        <Notice tone='info'>来自 {draft.sourceLabel}，共 {draft.preview.fullLength} 字。提交前请核对以下内容。</Notice>
        <Text className='wk-small'>{draft.preview.excerpt}</Text>
        <View className='wk-between'><View className='wk-tdesign-scope'>
          <t-button block size='large' theme='primary' ariaLabel='确认导入该职位' customStyle={tdesignButtonStyle} loading={importBusy.busy} onTap={() => void importBusy.run(async () => {
            const receipt = await career.confirmSharedImport(draft);
            setImportNotice(`已导入岗位 ${receipt.opportunityId}（${receipt.status === 'stored' ? '已存档' : '待复核'}）。`);
            setImported({ opportunityId: receipt.opportunityId, snapshotId: receipt.snapshotId });
            setDraft(undefined);
          })}>核对无误，导入</t-button>
        </View></View>
        <Action secondary onClick={() => setDraft(undefined)}>放弃本次导入</Action>
      </>}
      {imported && <Action secondary onClick={() => void navigate('careerApply', { opportunityId: imported.opportunityId, snapshotId: imported.snapshotId })}>就这个岗位继续：评估、申请与材料 ›</Action>}
      {!draft && <>
        {!entry && <Notice tone='info'>没有收到分享内容：可从微信聊天重新打开分享卡片，或在下方粘贴职位原文。</Notice>}
        <Field label='粘贴职位原文' value={pasteText} onChange={setPasteText} multiline placeholder='粘贴 JD 原文后先核对再导入' />
        <Action secondary disabled={!pasteText.trim()} onClick={() => { try { setDraft(career.prepareSharedImport(pasteText)); setPasteText(''); } catch (error) { setImportNotice(errorMessage(error)); } }}>生成核对预览</Action>
      </>}
      {importBusy.error && <Notice tone='danger'>{importBusy.error} 授权过期时请重新登录后再导入。</Notice>}
      {importNotice && <Notice tone='info'>{importNotice}</Notice>}
    </Card>

    <Action secondary onClick={() => void recoverBusy.run(async () => { await career.syncFromWeb(); query.reload(); })} loading={recoverBusy.busy}>同步 Web 端档案变更</Action>
    <Notice tone='info'>资格冲突、来源状态与待核实项全部如实展示；结果未知时可随时用原请求对账，不会重复执行。</Notice>
  </Screen>;
}
