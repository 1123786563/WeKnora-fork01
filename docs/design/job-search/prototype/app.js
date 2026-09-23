const jobs = [
  { company: '字节跳动', title: '前端开发工程师 · 校招', city: '北京', salary: '25–40K', fit: 92, tag: '资格符合', reason: 'React 项目、TypeScript 和组件设计经历与岗位要求直接对应。', gap: '需要补充一次性能优化的量化结果。', source: '企业校招官网', posted: '今天', accent: 'orange' },
  { company: '阿里云', title: 'AI 应用研发工程师 · 2027 届', city: '杭州', salary: '22–38K', fit: 86, tag: '资格符合', reason: '检索增强生成项目及 Python 服务经验覆盖核心职责。', gap: '模型评测经历需在简历中展开。', source: '企业校招官网', posted: '昨天', accent: 'blue' },
  { company: '腾讯', title: '产品经理 · 校招', city: '深圳', salary: '20–32K', fit: 78, tag: '待确认', reason: '用户研究与实习项目符合基础要求。', gap: '毕业时间字段与 JD 的 2027 届要求需要确认。', source: '用户粘贴的 JD', posted: '2 天前', accent: 'violet' }
];

const params = new URLSearchParams(location.search);
const state = {
  variant: ['A', 'B', 'C'].includes(params.get('variant')) ? params.get('variant') : 'C',
  platform: ['web', 'mobile', 'mini', 'all'].includes(params.get('platform')) ? params.get('platform') : 'all',
  view: 'discover', selectedJob: 0, savedJobs: [], watching: false, searchRuns: 1,
  materialVersion: 1, applicationStage: '待准备', prompt: '帮我找北京、上海的前端校招岗位，优先 AI 产品团队',
  notice: '演示数据已就绪', showState: false
};

const root = document.getElementById('prototype-root');
const esc = value => String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
const icon = name => ({ compass: '⌕', tasks: '▣', material: '▤', profile: '◉' }[name] || '✦');
const navItems = [['discover', '找岗位', 'compass'], ['applications', '申请', 'tasks'], ['materials', '材料', 'material'], ['profile', '我的', 'profile']];

function syncUrl() {
  const url = new URL(location.href);
  url.searchParams.set('variant', state.variant);
  url.searchParams.set('platform', state.platform);
  history.replaceState(null, '', url);
}

function platformTitle(platform) { return { web: 'Web 工作台', mobile: '移动 App', mini: '微信小程序' }[platform]; }
function job() { return jobs[state.selectedJob]; }
function actionButton(action, label, cls = '') { return '<button class="btn ' + cls + '" data-action="' + action + '">' + label + '</button>'; }
function pills() { return '<div class="pills"><span>北京 · 上海 · 深圳 · 杭州</span><span>2027 届</span><span>技术岗优先</span></div>'; }
function topLine() { return '<div class="eyebrow"><span class="eyebrow-mark"></span> WEKNORA CAREER <span class="muted">/ 求职空间</span></div>'; }

function shell(platform) {
  const web = platform === 'web';
  const mobile = platform === 'mobile';
  return '<section class="device ' + platform + '">' +
    (web ? '<header class="web-header"><div class="brand"><span class="brand-icon">W</span><strong>WeKnora</strong><span class="brand-suffix">Career</span></div><div class="web-header-right"><span class="header-date">2026 校招季</span><span class="avatar">林</span></div></header>' : '<div class="phone-status"><span>9:41</span><span>●●● ▰</span></div>' + (mobile ? '<div class="app-header"><span class="brand-mini">WeKnora <b>Career</b></span><span class="header-icon">◌</span></div>' : '<div class="mini-header"><span>‹</span><strong>求职空间</strong><span class="wechat-capsule">••• <i></i> ◉</span></div>')) +
    '<div class="device-layout">' + (web ? '<aside class="web-side"><div class="side-caption">我的空间</div>' + navItems.map(n => navButton(n, true)).join('') + '<div class="side-footer"><div class="tiny-orb">✦</div><strong>今日进度</strong><p>完成 2 项准备任务<br>继续保持节奏</p></div></aside>' : '') +
    '<main class="device-main">' + renderView(platform) + '</main></div>' +
    (web ? '' : '<nav class="bottom-nav">' + navItems.map(n => navButton(n, false)).join('') + '</nav>') +
    (mobile ? '<div class="home-indicator"></div>' : '') + '</section>';
}

function navButton(n, side) {
  return '<button class="nav-btn ' + (state.view === n[0] ? 'active' : '') + '" data-view="' + n[0] + '"><span class="nav-icon">' + icon(n[2]) + '</span><span>' + n[1] + '</span>' + (side && state.view === n[0] ? '<span class="nav-arrow">↗</span>' : '') + '</button>';
}

function renderView(platform) {
  if (state.view === 'profile') return profileView(platform);
  if (state.view === 'materials') return materialsView(platform);
  if (state.view === 'applications') return applicationsView(platform);
  return { A: workbenchView, B: journeyView, C: commandView }[state.variant](platform);
}

function pageHeading(kicker, title, subtitle) {
  return '<div class="page-heading"><div class="micro-label">' + kicker + '</div><h1>' + title + '</h1><p>' + subtitle + '</p></div>';
}

function jobRow(item, index, compact = false) {
  const selected = state.selectedJob === index;
  return '<button class="job-row ' + (selected ? 'selected ' : '') + (compact ? 'compact' : '') + '" data-job="' + index + '"><span class="company-mark ' + item.accent + '">' + item.company.slice(0, 1) + '</span><span class="job-text"><strong>' + item.title + '</strong><small>' + item.company + ' · ' + item.city + ' · ' + item.salary + '</small><span class="job-meta">' + item.source + ' · ' + item.posted + '</span></span><span class="fit-score">' + item.fit + '<small>%</small></span></button>';
}

function evidenceCard(compact = false) {
  const item = job();
  return '<div class="evidence-card ' + (compact ? 'compact' : '') + '"><div class="card-top"><span class="micro-label">匹配判断 · 有依据</span><span class="status-chip">' + item.tag + '</span></div><div class="evidence-score"><strong>' + item.fit + '%</strong><span>匹配度</span></div><div class="evidence-line"><span class="line-icon good">✓</span><div><strong>为什么推荐</strong><p>' + item.reason + '</p></div></div><div class="evidence-line"><span class="line-icon warn">!</span><div><strong>投递前确认</strong><p>' + item.gap + '</p></div></div><div class="source-line">来源：' + item.source + '　·　按原始 JD 核验</div></div>';
}

function quickActions() {
  return '<div class="action-row">' + actionButton('start', '准备这份申请 ↗', 'primary') + actionButton('save', state.savedJobs.includes(state.selectedJob) ? '已收藏 ✓' : '收藏岗位', 'subtle') + '</div>';
}

function importBar() {
  return '<div class="import-bar"><span class="import-icon">↗</span><div><strong>已有心仪岗位？</strong><p>粘贴招聘链接、JD，或从手机分享导入</p></div>' + actionButton('import', '导入岗位', 'light') + '</div>';
}

function workbenchView(platform) {
  const compact = platform !== 'web';
  return '<div class="variant-view workbench">' + topLine() + pageHeading('01 / 找到合适的岗位', '早上好，林同学<span class="title-dot">.</span>', '今天为你筛选出 3 个候选岗位，先看资格和证据，再决定要不要准备申请。') +
    '<div class="search-strip"><span>⌕</span><input aria-label="搜索岗位" placeholder="岗位、公司、城市或一句话指令" value="前端开发 · AI 产品团队" />' + actionButton('search', '搜索岗位', 'primary') + '</div>' + pills() +
    '<div class="workbench-grid"><section class="pane jobs-pane"><div class="pane-title"><div><span class="micro-label">FRESH MATCHES</span><h2>为你发现</h2></div><span class="count">3 个</span></div><div class="job-list">' + jobs.map((j, i) => jobRow(j, i, compact)).join('') + '</div><button class="text-action" data-action="search">重新筛选岗位 →</button></section>' +
    '<section class="pane details-pane"><div class="pane-title"><div><span class="micro-label">SELECTED ROLE</span><h2>岗位详情</h2></div><button class="icon-button" data-action="watch" aria-label="关注岗位">' + (state.watching ? '♥' : '♡') + '</button></div><div class="detail-head"><span class="company-mark large ' + job().accent + '">' + job().company.slice(0, 1) + '</span><div><h3>' + job().title + '</h3><p>' + job().company + ' · ' + job().city + ' · ' + job().salary + '</p></div></div>' + evidenceCard(true) + quickActions() + '</section>' +
    '<aside class="next-pane"><div class="next-card"><span class="micro-label">TODAY / 下一步</span><h2>把进度握在手里</h2><div class="next-stat"><strong>03</strong><span>待处理事项</span></div><div class="todo-item"><span class="todo-check">1</span><div><strong>确认个人资料</strong><small>毕业时间与城市偏好</small></div></div><div class="todo-item"><span class="todo-check">2</span><div><strong>生成岗位版简历</strong><small>PDF + 可编辑 DOCX</small></div></div><div class="todo-item"><span class="todo-check">3</span><div><strong>本人完成投递</strong><small>在招聘官网提交后记录状态</small></div></div><button class="text-action" data-view="applications">打开申请看板 →</button></div></aside></div>' + importBar() + '</div>';
}

function journeyView(platform) {
  return '<div class="variant-view journey">' + topLine() + pageHeading('你的校招旅程', '一步一步，走向下一份工作<span class="title-dot">.</span>', '从了解自己到跟进每次申请，所有重要动作都在同一条线上。') +
    '<div class="journey-hero"><div class="journey-copy"><span class="hero-kicker">当前阶段 / 02 OF 05</span><h2>找到值得投的岗位</h2><p>你的资料已经可以用于初步匹配。先挑选机会，再为每个岗位准备真实、具体的材料。</p>' + actionButton('search', '继续找岗位 →', 'inverse') + '</div><div class="orbit-art"><div class="orbit-ring r1"></div><div class="orbit-ring r2"></div><div class="orbit-core">02<span>/05</span></div><span class="orbit-star one">✦</span><span class="orbit-star two">✧</span></div></div>' +
    '<div class="journey-steps"><div class="step done"><span>01</span><strong>建立个人档案</strong><small>已完成</small></div><div class="step current"><span>02</span><strong>发现合适岗位</strong><small>进行中</small></div><div class="step"><span>03</span><strong>准备申请材料</strong><small>下一步</small></div><div class="step"><span>04</span><strong>本人完成投递</strong><small>待开始</small></div><div class="step"><span>05</span><strong>跟进每次进展</strong><small>待开始</small></div></div>' +
    '<div class="journey-sections"><section class="journey-section"><div class="section-head"><div><span class="micro-label">STEP 02 / 精选机会</span><h2>先从这份岗位开始</h2></div><span class="section-link">1 / 3</span></div><div class="spotlight-job"><div class="spotlight-top"><span class="company-mark large ' + job().accent + '">' + job().company.slice(0, 1) + '</span><span class="status-chip">' + job().tag + '</span></div><h3>' + job().title + '</h3><p>' + job().company + ' · ' + job().city + ' · ' + job().salary + '</p><div class="fit-bar"><span style="width:' + job().fit + '%"></span></div><div class="fit-copy"><strong>' + job().fit + '% 匹配</strong><small>基于你确认过的资料</small></div><div class="job-caveat"><strong>投递前核实：</strong>' + job().gap + '</div><div class="source-line">来源：' + job().source + '</div>' + quickActions() + '</div><div class="mini-job-list">' + jobs.map((j, i) => jobRow(j, i, true)).join('') + '</div></section><aside class="journey-section guide-section"><span class="micro-label">给你的建议</span><div class="quote-mark">“</div><h2>先确认资格，再投入准备时间。</h2><p>' + job().gap + '</p><div class="guide-links"><button data-view="profile">核对个人档案 ↗</button><button data-view="materials">查看申请材料 ↗</button></div></aside></div>' + importBar() + '</div>';
}

function commandView(platform) {
  return '<div class="variant-view command">' + topLine() + '<div class="command-heading"><span class="micro-label">你的求职副驾</span><h1>今天想推进什么<span class="title-dot">？</span></h1><p>用一句话说明目标。我们会把岗位、依据和下一步行动整理在一起。</p></div>' +
    '<div class="command-layout"><section class="conversation"><div class="prompt-bubble"><div class="prompt-avatar">林</div><p>' + esc(state.prompt) + '</p></div><div class="assistant-message"><span class="assistant-avatar">✦</span><div class="assistant-body"><div class="assistant-name">WeKnora <span>刚刚</span></div><p>我按你的条件整理了 <strong>3 个值得优先看的岗位</strong>。以下是目前最匹配的一份，资格判断基于你确认过的资料。</p><div class="answer-card"><div class="answer-title"><span class="company-mark ' + job().accent + '">' + job().company.slice(0, 1) + '</span><div><strong>' + job().title + '</strong><small>' + job().company + ' · ' + job().city + '</small></div><span class="fit-score">' + job().fit + '<small>%</small></span></div><div class="answer-reason"><span>' + job().tag + ' · 匹配依据</span><p>' + job().reason + '</p><p><strong>投递前核实：</strong>' + job().gap + '</p><div class="source-line">来源：' + job().source + '</div></div><div class="answer-actions">' + actionButton('start', '准备申请 ↗', 'primary') + actionButton('save', state.savedJobs.includes(state.selectedJob) ? '已收藏 ✓' : '先收藏', 'subtle') + '</div></div><div class="suggestion-row"><button data-action="prompt-next">还有哪些类似岗位？</button><button data-view="materials">帮我准备岗位简历</button><button data-view="applications">查看申请进度</button></div></div></div><form class="composer" id="command-form"><span>✦</span><input id="prompt-input" aria-label="求职指令" placeholder="例如：找上海的 AI 产品实习，适合 2027 届" /><button type="submit">发送 ↑</button></form><p class="composer-hint">支持粘贴招聘链接、JD 或直接描述求职目标。信息需自行核验。</p></section>' +
    '<aside class="command-rail"><div class="rail-card"><span class="micro-label">SEARCH SCOPE</span><h3>这次的筛选条件</h3><div class="condition"><span>岗位</span><strong>前端开发</strong></div><div class="condition"><span>城市</span><strong>北京 · 上海</strong></div><div class="condition"><span>方向</span><strong>AI 产品团队</strong></div><div class="condition"><span>毕业届别</span><strong>2027 届</strong></div><button class="text-action" data-view="profile">编辑我的偏好 →</button></div><div class="rail-card evidence-rail"><span class="micro-label">WHY THIS JOB</span><h3>推荐有迹可循</h3><p>' + job().reason + '</p><div class="source-line">来源：' + job().source + '</div></div><div class="rail-card small-rail"><span>◉</span><div><strong>你掌控最后一步</strong><p>申请材料由你确认，并在招聘平台本人投递。</p></div></div></aside></div></div>';
}

function profileView(platform) {
  return '<div class="secondary-view">' + topLine() + pageHeading('PERSONAL PROFILE', '确认你的求职档案<span class="title-dot">.</span>', 'AI 只使用你确认过的信息判断岗位资格和生成材料。') + '<div class="secondary-grid"><div class="white-card"><span class="micro-label">基本信息</span><h2>林同学 <span class="verified">✓ 已确认</span></h2><div class="info-grid"><div><small>毕业时间</small><strong>2027 年 6 月</strong></div><div><small>学历</small><strong>本科 · 计算机科学</strong></div><div><small>目标城市</small><strong>北京 / 上海 / 杭州</strong></div><div><small>目标方向</small><strong>前端 / AI 应用</strong></div></div></div><div class="white-card"><span class="micro-label">经历证据</span><h2>用真实经历说话</h2><p>2 段项目经历、1 段实习经历已整理。生成材料时会引用来源，并提示尚未核实的描述。</p><div class="tag-group"><span>React 组件库</span><span>RAG 项目</span><span>用户访谈</span></div></div></div>' + actionButton('confirm-profile', '确认档案信息', 'primary') + '</div>';
}

function materialsView(platform) {
  return '<div class="secondary-view">' + topLine() + pageHeading('APPLICATION MATERIALS', '材料准备，因岗而异<span class="title-dot">.</span>', '每个版本都可以回看来源，确认后再下载与投递。') + '<div class="secondary-grid"><div class="white-card material-preview"><span class="micro-label">岗位版简历 · V' + state.materialVersion + '</span><h2>林同学</h2><p>前端开发 / AI 应用方向</p><div class="resume-lines"><span></span><span></span><span></span><span></span></div><div class="resume-section"><strong>项目经历</strong><p>设计并实现可复用组件，服务 3 个业务页面；具体结果待本人核实。</p></div><div class="resume-section"><strong>技能</strong><p>React · TypeScript · Python · RAG</p></div></div><div class="white-card"><span class="micro-label">版本管理</span><h2>' + job().company + ' · ' + job().title + '</h2><p>简历内容基于个人档案与岗位要求生成。请核实措辞和指标。</p><div class="version-line"><span class="version-icon">▤</span><div><strong>岗位版简历 V' + state.materialVersion + '</strong><small>可下载 PDF 和 DOCX</small></div></div><div class="vertical-actions">' + actionButton('generate', '生成新版本 ↗', 'primary') + actionButton('pdf', '下载 PDF', 'subtle') + actionButton('docx', '下载可编辑 DOCX', 'subtle') + '</div></div></div></div>';
}

function applicationsView(platform) {
  return '<div class="secondary-view">' + topLine() + pageHeading('APPLICATION TRACKER', '每一次申请，都有下一步<span class="title-dot">.</span>', '材料、投递和进展收在一处。实际提交仍由你在招聘平台完成。') + '<div class="secondary-grid"><div class="white-card"><span class="micro-label">当前申请</span><div class="app-job-head"><span class="company-mark large ' + job().accent + '">' + job().company.slice(0, 1) + '</span><div><h2>' + job().title + '</h2><p>' + job().company + ' · ' + job().city + '</p></div></div><div class="stage-badge">' + state.applicationStage + '</div><div class="timeline"><div class="active"><span></span><div><strong>选定岗位</strong><small>已核验基本资格</small></div></div><div class="' + (state.applicationStage !== '待准备' ? 'active' : '') + '"><span></span><div><strong>准备申请材料</strong><small>简历、问答与岗位信息</small></div></div><div class="' + (state.applicationStage === '已投递' ? 'active' : '') + '"><span></span><div><strong>本人完成投递</strong><small>到招聘平台提交后在此记录</small></div></div></div></div><div class="white-card"><span class="micro-label">下一步行动</span><h2>保持信息准确</h2><p>完成投递后记录时间与渠道，后续可以跟进面试、笔试与结果。</p><div class="vertical-actions">' + actionButton('generate', '准备岗位版简历', 'primary') + actionButton('submitted', '我已在招聘平台投递', 'subtle') + actionButton('watch', state.watching ? '已开启提醒 ✓' : '关注后续进展', 'subtle') + '</div></div></div></div>';
}

function render() {
  syncUrl();
  const platforms = state.platform === 'all' ? ['web', 'mobile', 'mini'] : [state.platform];
  root.innerHTML = '<div class="prototype-page"><header class="prototype-toolbar"><div><span class="prototype-badge">THROWAWAY PROTOTYPE</span><h1>求职空间 · 三端交互方向</h1><p>同一产品能力，探索三种进入求职流程的方式。</p></div><div class="platform-tabs">' + [['all', '三端对照'], ['web', 'Web'], ['mobile', '移动端'], ['mini', '小程序']].map(p => '<button class="' + (state.platform === p[0] ? 'active' : '') + '" data-platform="' + p[0] + '">' + p[1] + '</button>').join('') + '</div></header><div class="variant-caption"><span class="caption-index">' + state.variant + '</span><div><strong>' + ({ A: '高密度工作台', B: '循序求职旅程', C: '对话指挥台' }[state.variant]) + '</strong><p>' + ({ A: '把岗位、资格证据和下一步行动放在同一视野。', B: '围绕阶段与任务组织找工作这件事。', C: '用自然语言发起行动，结果和依据跟随对话。' }[state.variant]) + '</p></div></div><div class="showcase ' + (state.platform === 'all' ? 'all' : 'single') + '">' + platforms.map(p => '<div class="device-wrap ' + p + '"><div class="device-label"><span class="label-dot"></span>' + platformTitle(p) + '</div>' + shell(p) + '</div>').join('') + '</div><div class="prototype-state"><button data-action="toggle-state">' + (state.showState ? '收起' : '展开') + '原型状态 ↗</button>' + (state.showState ? '<pre>' + esc(JSON.stringify(state, null, 2)) + '</pre>' : '<span>' + esc(state.notice) + '</span>') + '</div></div><div class="variant-switcher"><span>设计方向</span>' + ['A', 'B', 'C'].map(v => '<button class="' + (state.variant === v ? 'active' : '') + '" data-variant="' + v + '">' + v + '</button>').join('') + '<small>← →</small></div>';
}

function setNotice(message) { state.notice = message; render(); }
root.addEventListener('click', event => {
  const variant = event.target.closest('[data-variant]');
  if (variant) { state.variant = variant.dataset.variant; state.view = 'discover'; setNotice('已切换到方案 ' + state.variant); return; }
  const platform = event.target.closest('[data-platform]');
  if (platform) { state.platform = platform.dataset.platform; setNotice('正在查看' + (state.platform === 'all' ? '三端对照' : platformTitle(state.platform))); return; }
  const view = event.target.closest('[data-view]');
  if (view) { state.view = view.dataset.view; setNotice('已打开' + navItems.find(n => n[0] === state.view)[1]); return; }
  const picked = event.target.closest('[data-job]');
  if (picked) { state.selectedJob = Number(picked.dataset.job); setNotice('已查看 ' + job().company + ' 的岗位'); return; }
  const action = event.target.closest('[data-action]');
  if (!action) return;
  const kind = action.dataset.action;
  if (kind === 'toggle-state') { state.showState = !state.showState; render(); }
  else if (kind === 'search') { state.searchRuns++; setNotice('演示搜索完成：找到 ' + jobs.length + ' 个候选岗位'); }
  else if (kind === 'save') { const index = state.savedJobs.indexOf(state.selectedJob); index < 0 ? state.savedJobs.push(state.selectedJob) : state.savedJobs.splice(index, 1); setNotice(index < 0 ? '岗位已收藏' : '已取消收藏'); }
  else if (kind === 'watch') { state.watching = !state.watching; setNotice(state.watching ? '已关注后续进展' : '已取消关注'); }
  else if (kind === 'start') { state.applicationStage = '准备中'; state.view = 'applications'; setNotice('已创建演示申请任务'); }
  else if (kind === 'generate') { state.materialVersion++; state.view = 'materials'; setNotice('已生成演示材料版本 V' + state.materialVersion); }
  else if (kind === 'submitted') { state.applicationStage = '已投递'; setNotice('已记录本人投递状态（仅演示）'); }
  else if (kind === 'pdf' || kind === 'docx') setNotice('原型未生成真实文件：' + kind.toUpperCase() + ' 下载仅演示');
  else if (kind === 'import') setNotice('演示导入入口：支持招聘链接、JD 文本和移动端分享');
  else if (kind === 'confirm-profile') setNotice('个人档案已确认（仅演示）');
  else if (kind === 'prompt-next') { state.prompt = '还有哪些类似岗位？'; state.selectedJob = (state.selectedJob + 1) % jobs.length; setNotice('已展示另一个演示岗位'); }
});

root.addEventListener('submit', event => {
  if (event.target.id !== 'command-form') return;
  event.preventDefault();
  const input = event.target.querySelector('input');
  const value = input.value.trim();
  if (!value) return;
  state.prompt = value;
  state.searchRuns++;
  setNotice('已处理演示指令：' + value);
});

document.addEventListener('keydown', event => {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
  if (event.target.matches('input, textarea, [contenteditable]')) return;
  const variants = ['A', 'B', 'C'];
  state.variant = variants[(variants.indexOf(state.variant) + (event.key === 'ArrowRight' ? 1 : 2)) % 3];
  state.view = 'discover';
  setNotice('已切换到方案 ' + state.variant);
});

render();
