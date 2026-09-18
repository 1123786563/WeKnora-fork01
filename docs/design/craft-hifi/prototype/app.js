'use strict';
// Frontend-only, deterministic mock. No network, model, storage or credential access.
// ICONS is injected by build.py from a locally installed, licensed icon subset.
const $ = s => document.querySelector(s), $$ = s => Array.from(document.querySelectorAll(s));
const esc = v => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const icon = name => ICONS[name] || ICONS.file;
const types = { web: { label: '网页', icon: 'web', name: '季度经营分析看板', subtitle: '从业务数据到清晰、可追溯的经营洞察', file: 'index.html' }, document: { label: '文档', icon: 'file', name: '企业知识库应用实践指南', subtitle: '基于团队资料，整理可复用的实施指南', file: 'guide.md' }, spreadsheet: { label: '表格', icon: 'table', name: '项目预算与资源规划', subtitle: '汇总预算、费用与差异，保留计算口径', file: 'budget.xlsx' }, slides: { label: '演示稿', icon: 'slides', name: '产品知识与增长策略', subtitle: '把知识整理成结构清晰的汇报材料', file: 'strategy.pptx' } };
const projects = [{ id: 'p1', kind: 'web', title: '季度经营分析看板', desc: '销售指标、区域对比与有来源的业务结论', time: '今天 10:42', version: 3 }, { id: 'p2', kind: 'document', title: '企业知识库应用实践指南', desc: '知识治理、协作流程与落地方法', time: '今天 09:18', version: 2 }, { id: 'p3', kind: 'spreadsheet', title: '项目预算与资源规划', desc: '预算分配与成本差异明细', time: '昨天 16:30', version: 2 }, { id: 'p4', kind: 'slides', title: '产品知识与增长策略', desc: '面向团队的产品规划与方案汇报', time: '昨天 14:05', version: 1 }, { id: 'p5', kind: 'web', title: '产品使用反馈洞察', desc: '整理用户反馈，呈现问题分布与优先级', time: '09-16 18:20', version: 2 }, { id: 'p6', kind: 'document', title: '新成员入职协作手册', desc: '汇总团队资料，缩短新成员上手时间', time: '09-15 11:08', version: 1 }];
const scenarioInfo = { ready: ['已交付', '查看与继续修改', 'good'], running: ['生成中', '主任务正在执行', 'blue'], approval: ['等待授权', '批准或拒绝本次操作', 'warn'], question: ['等待回答', '澄清范围，不授予权限', 'blue'], reconnecting: ['连接中断', '恢复读取，不重发任务', 'warn'], failed: ['本轮失败', '上一交付版本仍然可用', 'bad'], canceled: ['已取消', '保留已交付作品', ''], readonly: ['只读访问', '可查看，不可提交修改', ''], expired: ['预览已过期', '仅续期预览授权', 'warn'], unknown: ['正在核对', '执行结果不明，不自动重试', 'warn'] };
const state = { route: 'home', kind: 'web', title: types.web.name, scenario: 'ready', draft: '', homeDraft: '', filter: 'all', search: '', tab: 'preview', mobileTab: 'chat', viewport: 'desktop', viewVersion: 3, baseVersion: 3, workspaceRevision: 7, runCount: 1, versions: [{ id: 3, label: '补充区域筛选与来源说明', time: '今天 10:42', restorable: true, sourceIds: ['S1', 'S2'] }, { id: 2, label: '新增季度销售趋势与指标卡片', time: '今天 10:28', restorable: true, sourceIds: ['S1', 'S2'] }, { id: 1, label: '生成第一版经营报告', time: '今天 10:16', restorable: true, sourceIds: ['S1', 'S2'] }], lastPrompt: '结合销售数据和产品知识，生成季度经营分析看板。需要汇总指标、区域筛选，以及可追溯的业务结论。', selectedSources: ['S1', 'S2'], uploaded: [], sourcesRevoked: false, decision: 'pending', slide: 1, region: 'all', drawer: null, restoreTarget: null };
let toastTimer, returnFocus;
function btn(text, action, cls = '', attrs = '') { return `<button class="btn ${cls}" data-action="${action}" ${attrs}>${text}</button>`; }
function badge(text, cls = '') { return `<span class="badge ${cls}">${text}</span>`; }
function current() { return types[state.kind]; }
function seriesValues() { const f = (state.region === 'east' ? .61 : state.region === 'west' ? .39 : 1) * (state.viewVersion === 1 ? 120.5 / 128.6 : state.viewVersion === 2 ? 124.8 / 128.6 : 1); return [16.2, 18.5, 19.4, 21.9, 24.6, 28].map(v => v * f); }
function versionSources() { if (state.route !== 'workbench')
    return state.selectedSources; return state.versions.find(v => v.id === state.viewVersion)?.sourceIds || state.activeSourceIds || state.selectedSources; }
function sourceCaption() { return versionSources().map(id => id === 'S1' ? '销售数据.csv [S1]' : '产品知识库 [S2]').join(' · ') || '本版本未关联示例来源'; }
function isBusy() { return ['running', 'approval', 'question', 'unknown'].includes(state.scenario); }
function canWrite() { return state.scenario !== 'readonly'; }
function canSend() { return canWrite() && !isBusy() && state.scenario !== 'reconnecting'; }
function showToast(text) { const t = $('#toast'); t.textContent = text; t.classList.add('visible'); clearTimeout(toastTimer); toastTimer = setTimeout(() => t.classList.remove('visible'), 4400); }
function go(route) { closeDrawer(); state.route = route; document.body.classList.remove('nav-open'); if (location.hash.slice(1) !== route)
    history.pushState(null, '', '#' + route); render(); }
function sidebar() { return `<button class="nav-shade" data-action="close-nav" aria-label="关闭导航"></button><aside class="sidebar" aria-label="主导航"><a class="brand" href="#home" data-route="home">${icon('layers')}<div><div class="brand-word">WeKnora</div><div class="brand-sub">KNOWLEDGE WORKSPACE</div></div></a><button class="space-switch" data-action="space"><span class="avatar">产</span><span class="grow"><b>产品团队</b><span class="small muted">企业工作空间 · 演示</span></span>${icon('chevron')}</button>${btn(icon('plus') + '<span>新建作品</span>', 'new', 'action full new-btn')}<nav class="nav"><a href="#home" data-route="home" class="${state.route === 'home' ? 'active' : ''}">${icon('home')}<span>创作首页</span></a><a href="#library" data-route="library" class="${state.route === 'library' ? 'active' : ''}">${icon('grid')}<span>我的作品</span>${badge('6')}</a><a href="#templates" data-route="templates" class="${state.route === 'templates' ? 'active' : ''}">${icon('template')}<span>创作模板</span></a></nav><div class="nav-label">资料与协作</div><nav class="nav"><a href="#sources" data-action="sources">${icon('book')}<span>知识与资料</span></a><a href="#workbench" data-action="open-project" data-id="p1" class="${state.route === 'workbench' ? 'active' : ''}">${icon('message')}<span>当前创作</span></a></nav><div class="nav-label">最近打开</div><div class="recent-nav">${projects.slice(0, 3).map(p => `<a href="#workbench" data-action="open-project" data-id="${p.id}">${icon(types[p.kind].icon)}<span class="truncate">${p.title}</span></a>`).join('')}</div><div class="sidebar-bottom"><div class="scope-note"><div class="flex" style="margin-bottom:5px;color:var(--craft-success-text)">${icon('shield')}<b>在授权范围内创作</b></div>资料按权限使用，作品按版本保存。</div><nav class="nav"><a href="#design" data-route="design" class="${state.route === 'design' ? 'active' : ''}">${icon('paint')}<span>设计规范</span></a></nav><div class="profile"><span class="avatar">示</span><div class="grow"><b>演示用户</b><div class="tiny muted">高保真原型 · 非生产系统</div></div>${btn(icon('settings'), 'scenario', 'ghost icon-only', 'aria-label="打开原型演示控制" title="演示控制"')}</div></div></aside>`; }
function header() { const work = state.route === 'workbench'; return `<header class="topbar"><div class="breadcrumb">${btn(icon('menu'), 'menu', 'ghost icon-only mobile-menu', 'aria-label="打开导航"')}<span class="crumb-root">工作空间</span><span class="slash">/</span><b>${work ? esc(state.title) : ({ home: '创作', library: '我的作品', templates: '创作模板', design: '设计规范' }[state.route] || '创作')}</b>${work ? badge(scenarioInfo[state.scenario][0], scenarioInfo[state.scenario][2]) : ''}</div><div class="top-actions">${work ? `${btn(icon('book') + '<span class="desktop-label">来源</span> ' + badge(String(versionSources().length)), 'sources', 'ghost', 'aria-label="查看知识来源"')}${btn(icon('clock') + '<span class="desktop-label">版本记录</span>', 'versions', '', 'aria-label="查看版本记录"')}${btn(icon('download') + '<span class="desktop-label">导出作品</span>', 'export', 'blue', state.viewVersion === null ? 'disabled aria-label="尚无版本可导出"' : 'aria-label="导出当前版本"')}` : '<span class="demo-tag">原型 · 模拟数据</span>'}${btn(icon('settings'), 'scenario', 'ghost icon-only', 'title="交互状态演示" aria-label="交互状态演示"')}</div></header>`; }
function typeButtons() { return `<div class="type-row" role="group" aria-label="作品类型">${Object.entries(types).map(([k, v]) => `<button class="type-btn ${state.kind === k ? 'active' : ''}" data-action="kind" data-kind="${k}" aria-pressed="${state.kind === k}">${icon(v.icon)}${v.label}</button>`).join('')}</div>`; }
function home() { return `${header()}<section class="home"><div class="home-title"><div class="eyebrow">${icon('spark')} WEKNORA CRAFT</div><h1>把知识，变成<span>作品。</span></h1><p>描述你的想法，连接团队资料。让 AI 帮你完成从内容到交付的每一步。</p></div><section class="creator" aria-label="创建作品">${typeButtons()}<textarea id="home-prompt" aria-label="描述创作目标" placeholder="例如：结合销售数据与产品知识，生成一份可交互的季度经营报告…">${esc(state.homeDraft)}</textarea><div class="input-resources">${state.selectedSources.includes('S1') ? `<button class="chip" data-action="sources">${icon('table')}销售数据.csv</button>` : ''}${state.selectedSources.includes('S2') ? `<button class="chip" data-action="sources">${icon('book')}产品知识库</button>` : ''}${state.uploaded.map(f => `<span class="chip">${icon('file')}${esc(f.name)}</span>`).join('')}</div><div class="creator-footer"><div class="flex">${btn(icon('attach') + '上传资料', 'upload', 'ghost')}${btn(icon('book') + '选择知识', 'choose-sources', 'ghost')}</div><div class="flex"><span class="small muted model-label">工作空间默认模型</span>${btn('开始创作 ' + icon('right'), 'create', 'action', 'id="create-btn"')}</div></div></section><div class="home-meta">${icon('shield')}<span>仅使用已授权资料 · 支持持续修改与版本追溯 · 当前资料与结果均为演示数据</span></div><div class="section-head"><div><h2>从一个好想法开始</h2></div>${btn('全部模板 ' + icon('right'), 'templates', 'text sm')}</div><div class="template-strip">${templateItem('web', '经营分析报告', '呈现数据，辅助业务复盘')}${templateItem('document', '团队知识手册', '让经验成为可复用的指南')}${templateItem('slides', '产品方案汇报', '把想法讲清楚，形成共识')}</div><div class="section-head"><div><h2>继续你的创作</h2><p>保留每一次修改，让好想法持续生长</p></div>${btn('查看全部 ' + icon('right'), 'library', 'text sm')}</div><div class="cards">${projects.slice(0, 3).map(card).join('')}</div><div class="home-bottom"><span>${icon('check')}知识与资料可追溯</span><span>${icon('clock')}历史版本独立保存</span><span>${icon('lock')}复用工作空间权限</span></div></section>`; }
function templateItem(kind, title, desc) { return `<button class="template-item" data-action="template" data-kind="${kind}"><span class="type-icon ${kind}">${icon(types[kind].icon)}</span><span class="grow"><b>${title}</b><p>${desc}</p></span>${icon('right')}</button>`; }
function miniature(kind) { if (kind === 'web')
    return `<div class="mini-content"><h4>季度经营概览</h4><div class="mini-stats"><div>销售收入<b>128.6万</b></div><div>订单数量<b>2,048</b></div><div>回购率<b>36.8%</b></div></div><canvas data-chart="mini" aria-label="销售趋势缩略图"></canvas></div>`; if (kind === 'document')
    return `<div class="mini-content"><h4>企业知识库应用实践指南</h4><p>KNOWLEDGE MANAGEMENT · TEAM GUIDE</p><h4 style="margin-top:12px">01 / 让知识进入工作流</h4><p>统一资料入口，保留版本与来源。在团队协作中持续沉淀可复用的知识。</p><p>连接已有文档，建立明确的访问范围与维护责任。</p><div class="mini-page-no">团队协作实践 · 版本 2</div></div>`; if (kind === 'spreadsheet')
    return `<div class="mini-content"><h4>项目预算与资源规划</h4><table class="mini-table"><tr><th>项目</th><th>预算</th><th>实际</th><th>差异</th></tr>${[['产品研发', '180,000', '165,000', '15,000'], ['平台服务', '42,000', '39,500', '2,500'], ['市场推广', '65,000', '59,000', '6,000'], ['合计', '287,000', '263,500', '23,500']].map(r => '<tr>' + r.map(c => `<td>${c}</td>`).join('') + '</tr>').join('')}</table></div>`; return `<div class="mini-slide"><small>WEKNORA / PRODUCT STRATEGY</small><h4>让知识创造<br>更大的价值。</h4><p>产品知识与增长策略 · 2026</p></div>`; }
function card(p) { return `<button class="artifact-card" data-action="open-project" data-id="${p.id}" data-kind="${p.kind}" aria-label="打开${p.title}"><div class="card-preview">${miniature(p.kind)}</div><div class="card-content"><div class="between"><span class="small muted flex">${icon(types[p.kind].icon)}${types[p.kind].label}</span>${badge('已交付', 'good')}</div><h3>${p.title}</h3><div class="card-description">${p.desc}</div><div class="card-footer"><span>${p.time}</span><span>v${p.version} · 仅团队可见</span></div></div></button>`; }
function filteredProjects() { return projects.filter(p => (state.filter === 'all' || p.kind === state.filter) && p.title.includes(state.search)); }
function library() { return `${header()}<section class="content-page"><div class="page-title"><div><h1>我的作品</h1><p>管理你的创作成果，随时回到上一次灵感。</p></div>${btn(icon('plus') + '新建作品', 'new', 'action')}</div><div class="toolbar"><div class="segments" role="group" aria-label="按类型筛选">${[['all', '全部'], ...Object.entries(types).map(([k, v]) => [k, v.label])].map(([k, n]) => `<button data-action="filter" data-filter="${k}" class="${state.filter === k ? 'active' : ''}" aria-pressed="${state.filter === k}">${n}</button>`).join('')}</div><label class="search-box">${icon('search')}<input id="project-search" placeholder="搜索作品名称" aria-label="搜索作品" value="${esc(state.search)}"></label></div><div class="cards" id="project-grid">${projectResults()}</div><p class="small muted" style="margin-top:24px;text-align:center">当前展示 6 件示例作品；真实列表由 Craft 会话接口按权限返回。</p></section>`; }
function projectResults() { const list = filteredProjects(); return list.length ? list.map(card).join('') : `<div class="empty">${icon('search')}<h3>没有找到匹配的作品</h3><p>试试更短的关键词，或清除当前筛选。</p>${btn('清除筛选', 'clear-filter', '')}</div>`; }
function templates() { return `${header()}<section class="content-page"><div class="page-title"><div><div class="eyebrow" style="margin-bottom:7px">START WITH A PURPOSE</div><h1>从灵感，到第一版作品</h1><p>模板只提供创作目标，不扩大工具权限或自动启用未验收的能力。</p></div></div><div class="cards">${Object.entries(types).map(([k, v]) => `<article class="artifact-card" style="cursor:default"><div class="card-preview">${miniature(k)}</div><div class="card-content"><div class="small muted flex">${icon(v.icon)}${v.label}模板</div><h3>${v.name}</h3><p class="card-description">${v.subtitle}</p><div class="card-footer">${badge('示例模板')}${btn('使用模板 ' + icon('right'), 'template', 'text sm', `data-kind="${k}"`)}</div></div></article>`).join('')}</div><div class="notice" style="margin-top:24px">${icon('info')}<div><b>模板与运行能力分开管理</b>正式接入后，文档 / 表格 / 演示稿按服务端能力门禁独立开放；本页用于演示目标体验。</div></div></section>`; }
function scenarioNotice() { const s = state.scenario; if (s === 'reconnecting')
    return `<div class="notice warn">${icon('refresh')}<div class="grow"><b>连接已中断，正在同步原任务</b>你的修改不会被重复提交。</div>${btn('重新连接', 'reconnect', 'sm')}</div>`; if (s === 'unknown')
    return `<div class="notice warn">${icon('clock')}<div><b>正在核对执行结果</b>远端可能已接受任务。先读取确认，不自动重发。</div></div>`; if (s === 'failed')
    return `<div class="notice bad">${icon('warning')}<div class="grow"><b>本轮检查未通过</b>上一交付版本仍可查看和下载。</div>${btn('查看原因', 'checks', 'sm')}</div>`; if (s === 'canceled')
    return `<div class="notice">${icon('stop')}<div><b>当前任务已取消</b>未发布的变更没有覆盖历史版本。</div></div>`; if (s === 'readonly')
    return `<div class="notice">${icon('lock')}<div><b>当前是只读访问</b>可以查看作品，不能修改、恢复或批准操作。</div></div>`; return ''; }
function runCard() { const busy = isBusy(); const interrupted = ['failed', 'canceled', 'unknown', 'reconnecting'].includes(state.scenario); const label = interrupted ? scenarioInfo[state.scenario][0] : busy ? '进行中' : '已完成'; return `<section class="run-card"><header><b class="flex">${icon('bolt')}生成与验证作品</b>${badge(label, interrupted ? 'warn' : busy ? 'blue' : 'good')}</header><div class="step-list"><div class="step-row">${icon('check')}读取授权资料<small>${(state.activeSourceIds || state.selectedSources).length} 项来源</small></div><div class="step-row">${icon('check')}委派创作任务<small>OpenCode</small></div><div class="step-row">${icon(busy || interrupted ? 'clock' : 'check')}生成文件与内容检查<small>${interrupted ? '本轮未确认' : busy ? '等待结果' : '3 项通过'}</small></div><div class="step-row">${icon(busy || interrupted ? 'clock' : 'check')}保存独立作品版本<small>${busy || interrupted ? '本轮未发布' : 'v' + state.baseVersion}</small></div></div><details class="run-details"><summary>查看执行详情与任务标识</summary><p>run_demo_${state.runCount} · task_demo_1<br>主运行与委派任务分开记录；此处不展示模型内部推理。</p></details>${state.scenario === 'running' ? `<div style="padding:0 12px 12px">${btn('完成本轮演示', 'finish', 'sm full', 'aria-label="完成本轮演示"')}</div>` : ''}</section>`; }
function approval() { if (state.scenario === 'question')
    return `<section class="approval question"><h3 class="flex">${icon('message')}需要确认你的创作范围</h3><p>这份报告需要呈现哪个时间范围？</p><div class="flex wrap">${btn('最近一个季度', 'answer', 'sm blue')}${btn('最近半年', 'answer', 'sm')}</div><div class="tiny muted" style="margin-top:10px">回答只澄清需求，不代表允许执行命令。</div></section>`; if (state.scenario !== 'approval')
    return ''; return `<section class="approval"><h3 class="flex">${icon('shield')}执行前需要你的授权</h3><p>为验证作品，需要在当前隔离工作区执行构建命令。</p><code>npm run build<br>目录：/workspace/craft-demo</code><p class="tiny">权限范围：仅本次构建 · 请求 apr_demo_07<br>不包括安装依赖、外网访问或发布部署。</p>${state.decision === 'pending' ? `<div class="flex wrap">${btn('仅本次允许', 'approve', 'action sm')}${btn('拒绝', 'reject', 'sm')}</div>` : `<div class="notice warn" style="margin:12px 0 0"><div><b>决定已记录，等待远端确认</b>尚不能认定命令已经执行。</div></div>${btn('模拟远端确认送达', 'ack', 'sm full')}`}</section>`; }
function thread() { return `<section class="thread" data-mobile-hidden="${state.mobileTab !== 'chat'}" aria-label="创作对话"><div class="pane-head"><h3 class="flex">${icon('message')}创作对话</h3><span class="tiny muted">Craft Agent</span></div><div class="thread-scroll"><div class="thread-date">今天 · ${state.runCount > 1 ? '本轮修改' : '10:40'}</div>${scenarioNotice()}<div class="user-message">${esc(state.lastPrompt)}<div class="attachments">${state.selectedSources.includes('S1') ? `<span class="chip">${icon('table')}销售数据.csv</span>` : ''}${state.selectedSources.includes('S2') ? `<span class="chip">${icon('book')}产品知识库</span>` : ''}</div></div><div class="agent-label"><span class="agent-logo">${icon('spark')}</span>WeKnora Craft<span class="tiny muted" style="margin-left:auto">主 Agent</span></div><div class="assistant-text"><p>${isBusy() ? '我会先核对数据与业务口径，再生成可预览的作品。事实、计算结果和分析建议会分别标注。' : ({ web: '作品已准备好。我把业务指标、趋势和来源整理在一起，方便你继续核对与修改。', document: '指南已准备好。内容按摘要、协作方法和维护流程组织，保留资料引用，便于继续审核。', spreadsheet: '预算表已准备好。预算、实际支出与差异分栏展示，方便核对计算口径与明细。', slides: '演示稿已准备好。内容按目标、关键洞察、实施路径与行动建议组织，可以逐页查看。' }[state.kind])}</p></div>${runCard()}${approval()}${state.viewVersion !== null ? `<button class="delivery" data-action="preview"><span class="type-icon ${state.kind}">${icon(current().icon)}</span><span class="grow"><b>${esc(state.title)}</b><p>v${state.baseVersion} · 来源 ${state.selectedSources.length} 项 · 文件 4 个</p></span>${icon('external')}</button><p class="tiny muted">原型数字均为示例，不代表真实经营结果。</p>` : ''}<div class="suggestions">${({ web: ['补充环比分析', '精简结论表达', '突出关键指标'], document: ['补充实施步骤', '精简章节结构', '补全资料引用'], spreadsheet: ['补充预算说明', '核对差异公式', '调整金额格式'], slides: ['精简每页文字', '补充行动计划', '统一章节风格'] }[state.kind]).map(t => `<button data-action="suggestion" data-text="${t}">${t}</button>`).join('')}</div></div><div class="composer-wrap"><div class="composer"><textarea id="composer" aria-label="输入修改要求" placeholder="继续修改，例如：增加季度对比，保留来源…" ${canWrite() ? '' : 'disabled'}>${esc(state.draft)}</textarea><div class="composer-tools"><div class="flex">${btn(icon('attach'), 'upload', 'ghost icon-only', 'aria-label="添加资料"')}${btn(icon('book') + '资料', 'sources', 'ghost')}</div>${isBusy() ? btn(icon('stop') + '停止', 'stop', 'sm') : btn('发送 ' + icon('send'), 'send', 'action sm', canSend() ? '' : 'disabled')}</div></div><div class="base-meta"><span>编辑基线 ${state.baseVersion === null ? '未生成' : 'v' + state.baseVersion} · 同一工作区串行修改</span><span>Shift + Enter 换行</span></div></div></section>`; }
function report() { const factor = state.region === 'east' ? .61 : state.region === 'west' ? .39 : 1; const total = (state.viewVersion === 1 ? 120.5 : state.viewVersion === 2 ? 124.8 : 128.6) * factor; return `<article class="report ${state.viewport === 'mobile' ? 'narrow' : ''}" id="exportable-preview"><div class="report-nav"><b class="flex">${icon('chart')}经营观察</b><label>区域 <select id="region" aria-label="预览区域筛选"><option value="all" ${state.region === 'all' ? 'selected' : ''}>全部区域</option><option value="east" ${state.region === 'east' ? 'selected' : ''}>华东地区</option><option value="west" ${state.region === 'west' ? 'selected' : ''}>其他区域</option></select></label></div><div class="report-body"><div class="report-heading"><div class="eyebrow">QUARTERLY BUSINESS REVIEW</div><h2>让业务增长，有据可循。</h2><p>季度经营分析 · 数据更新至 2026 年 6 月 · 演示数据</p></div><div class="report-stats"><div class="metric"><p>销售收入</p><strong>¥ ${total.toFixed(1)}<span style="font-size:12px">万</span></strong><small>${state.viewVersion === 1 ? '第一版样例' : '较上期 +12.4%'}</small></div><div class="metric"><p>订单数量</p><strong>${Math.round(2048 * factor).toLocaleString('en-US')}</strong><small>较上期 +8.2%</small></div><div class="metric"><p>客户回购率</p><strong>36.8<span style="font-size:12px">%</span></strong><small>较上期 +3.1pp</small></div></div><div class="report-chart"><div class="between"><h3>销售趋势</h3><div class="chart-legend"><span><i class="legend-mark"></i>本期</span><span><i class="legend-mark green"></i>上期</span></div></div><canvas data-chart="main" aria-label="1月至6月销售趋势示例，金额单位万元" role="img"></canvas></div><div class="insight">${icon('spark')}<div><b>值得关注的变化</b><p>本期销售表现持续增长，6 月贡献较高。建议结合产品结构进一步核对增长来源。<button class="btn text" style="font-size:10px;min-height:20px;padding:0 3px" data-action="sources">[S1] [S2]</button></p></div></div><div class="report-footer"><span>来源：${esc(sourceCaption())}</span><span>内容版本 v${state.viewVersion}</span></div></div></article>`; }
function documentPreview() { return `<article class="document-paper" id="exportable-preview"><div class="eyebrow" style="color:var(--craft-brand-strong)">KNOWLEDGE / TEAM GUIDE</div><h1>企业知识库<br>应用实践指南</h1><div class="doc-info">产品团队 · v${state.viewVersion} · 内部协作材料 / 原型示例</div><h2>摘要</h2><p>知识库的价值不止于存储文档，更在于把正确的知识带入日常工作。本指南围绕资料接入、访问权限、业务引用与持续维护，建立可落地的协作方法。</p><blockquote>先明确知识的使用场景，再设计分类、责任与更新流程。所有结论应能回到对应资料。</blockquote><h2>01 / 让知识进入工作流</h2><p>将产品资料、规范与常见问题连接到同一工作空间。创作时选择需要的资料范围，保留来源定位与版本；审核时区分原文事实、计算结果与模型推断。<button class="btn text sm" data-action="sources">[S2]</button></p><h2>02 / 从资料到交付的协作闭环</h2><table class="data-table"><tr><th>阶段</th><th>关键活动</th><th>完成标准</th></tr><tr><td>资料准备</td><td>核对来源与授权</td><td>输入范围明确</td></tr><tr><td>创作与校验</td><td>生成、检查、人工审核</td><td>检查有证据</td></tr><tr><td>版本交付</td><td>固定内容、引用与文件</td><td>历史版本不变</td></tr></table><h2>03 / 持续维护，而非一次整理</h2><p>资料负责人定期核对有效性。权限变更后，后续检索与原文打开应重新授权，不沿用过去的访问许可。</p><div class="report-footer"><span>资料依据：产品知识库 [S2]</span><span>01 / 01 · 文档预览示例</span></div></article>`; }
function spreadsheetPreview() { return `<div id="exportable-preview"><div class="sheet-tools"><span class="mono">D6</span><span class="muted">fx</span><code>=SUM(D3:D5)</code><span class="tiny muted" style="margin-left:auto">只读预览 · 示例</span></div><div class="table-wrap" style="border-radius:0"><table class="data-table sheet"><tr><th></th><th>A</th><th>B</th><th>C</th><th>D</th></tr><tr><th>1</th><td colspan="4" style="font-weight:600;font-size:14px;padding-block:18px">项目预算与资源规划 / v${state.viewVersion}</td></tr><tr class="sheet-header"><th>2</th><td>费用类别</td><td>计划预算 / 元</td><td>实际费用 / 元</td><td>剩余额度 / 元</td></tr>${[['产品研发', 180000, 165000], ['平台服务', 42000, 39500], ['市场推广', 65000, 59000]].map((r, i) => `<tr><th>${i + 3}</th><td>${r[0]}</td><td>${r[1].toLocaleString()}</td><td>${r[2].toLocaleString()}</td><td>${(r[1] - r[2]).toLocaleString()}</td></tr>`).join('')}<tr class="total"><th>6</th><td>合计</td><td>287,000</td><td>263,500</td><td>23,500</td></tr>${[7, 8, 9, 10, 11].map(n => `<tr><th>${n}</th><td>&nbsp;</td><td></td><td></td><td></td></tr>`).join('')}</table></div><div class="sheet-foot">预算概览 <span class="tiny muted" style="margin-left:16px">原型算术演示，不代表真实 XLSX 公式引擎已验收</span></div></div>`; }
function slidesPreview() { const titles = ['让知识创造\n更大的价值。', '从分散信息，\n到可复用知识', '围绕真实任务，\n建立创作闭环', '让每一次交付，\n都可追溯']; return `<div class="slide-layout"><div class="slide-thumbs" aria-label="选择幻灯片">${[1, 2, 3, 4].map(n => `<button class="${state.slide === n ? 'active' : ''}" data-action="slide" data-slide="${n}" aria-label="第${n}页">${String(n).padStart(2, '0')}<br>${['封面', '知识', '创作', '交付'][n - 1]}</button>`).join('')}</div><div class="grow"><article class="slide ${state.slide > 1 ? 'alt' : ''}" id="exportable-preview"><small>WEKNORA / PRODUCT STRATEGY</small><div><h1>${titles[state.slide - 1].replace('\n', '<br>')}</h1>${state.slide === 1 ? '<p>产品知识与增长策略<br>产品团队 · 2026 / 原型示例</p>' : `<div class="slide-points"><div>01　以明确的业务目标组织资料</div><div>02　把来源、权限与版本带入工作流</div><div>03　先验证交付，再持续优化体验</div></div>`}</div><div class="slide-bottom"><span>产品团队 · 内容版本 v${state.viewVersion}</span><span>${String(state.slide).padStart(2, '0')} / 04</span></div></article><div class="between" style="margin-top:13px">${btn(icon('back') + '上一页', 'prev-slide', 'sm', state.slide === 1 ? 'disabled' : '')}${badge('4 页 HTML 演示，不是实际 PPTX')}${btn('下一页' + icon('right'), 'next-slide', 'sm', state.slide === 4 ? 'disabled' : '')}</div></div></div>`; }
function filesView() { const files = [{ name: current().file, size: '18.4 KB', mime: state.kind === 'web' ? 'text/html' : state.kind === 'document' ? 'text/markdown' : 'Office 文件' }, { name: 'assets/chart-data.json', size: '3.2 KB', mime: 'application/json' }, { name: 'sources.json', size: '1.1 KB', mime: 'application/json' }, { name: 'manifest.json', size: '2.6 KB', mime: 'application/json' }]; return `<div class="table-wrap"><div class="files-meta"><span class="type-icon">${icon('folder')}</span><div class="grow"><h3>版本 v${state.viewVersion} 的交付文件</h3><p class="tiny muted">文件清单与校验值固定于该版本</p></div>${badge('4 个文件')}</div><table class="data-table"><tr><th>文件路径</th><th>大小</th><th>操作</th></tr>${files.map(f => `<tr><td><span class="flex">${icon('file')}<span>${f.name}<small class="tiny muted" style="display:block">${f.mime}</small></span></span></td><td>${f.size}</td><td>${btn('查看元信息', 'file', 'text sm', `data-file="${f.name}"`)}</td></tr>`).join('')}</table></div><div class="help-block">此处是固定版本的文件清单示例。正式开发必须从 manifest 读取路径、大小与 SHA-256；不能把可变工作区目录直接作为历史版本下载入口。</div>`; }
function checksView() { return `<div class="checks-grid">${[['入口与资源检查', '入口文件及引用资源可定位', 'good', '通过'], ['构建与渲染检查', state.scenario === 'failed' ? '模拟：构建步骤失败，需要显式修复' : '模拟：构建结果与预览入口一致', state.scenario === 'failed' ? 'bad' : 'good', state.scenario === 'failed' ? '失败' : '通过'], ['内容与来源检查', '结果与所选来源标识关联', 'good', '通过'], ['真实模型与沙箱验收', '独立 HTML 原型没有执行真实模型、沙箱或 Office 工具', '', '未运行']].map(([n, d, c, s]) => `<article class="check-card"><div class="between"><h3>${n}</h3>${badge(s, c)}</div><p>${d}</p></article>`).join('')}</div><dl class="dev-dl"><dt>会话 / session</dt><dd>craft_demo_session</dd><dt>主运行 / run</dt><dd>run_demo_${state.runCount}</dd><dt>委派 / task</dt><dd>task_demo_1</dd><dt>工作区 revision</dt><dd>${state.workspaceRevision}</dd><dt>查看 / 编辑基线</dt><dd>v${state.viewVersion} / v${state.baseVersion}</dd><dt>环境</dt><dd>Frontend-only demonstration</dd></dl>`; }
function artifact() { let content; if (state.viewVersion === null)
    content = `<div class="empty">${icon('spark')}<h3>你的作品将在这里呈现</h3><p>生成、检查完成后，才会发布第一个版本。</p>${state.scenario === 'running' ? btn('完成本轮演示', 'finish', 'action') : ''}</div>`;
else if (state.tab === 'files')
    content = filesView();
else if (state.tab === 'checks')
    content = checksView();
else if (state.scenario === 'expired')
    content = `<div class="empty">${icon('lock')}<h3>预览授权已过期</h3><p>作品仍然存在，刷新授权即可继续查看。</p>${btn(icon('refresh') + '刷新预览', 'refresh-preview', 'blue')}</div>`;
else
    content = ({ web: report, document: documentPreview, spreadsheet: spreadsheetPreview, slides: slidesPreview }[state.kind])(); return `<section class="artifact-pane" data-mobile-hidden="${state.mobileTab !== 'preview'}" aria-label="作品区域"><div class="artifact-toolbar"><div class="tabs" role="tablist" aria-label="作品视图">${[['preview', '预览'], ['files', '文件'], ['checks', '检查 / 详情']].map(([k, t]) => `<button role="tab" aria-selected="${state.tab === k}" data-action="tab" data-tab="${k}" class="${state.tab === k ? 'active' : ''}">${t}</button>`).join('')}</div><select id="version-select" aria-label="选择查看版本" ${state.versions.length ? '' : 'disabled'}>${state.versions.length ? state.versions.map((v, i) => `<option value="${v.id}" ${state.viewVersion === v.id ? 'selected' : ''}>v${v.id} · ${i === 0 ? '最新交付' : '历史版本'}</option>`).join('') : '<option>暂无版本</option>'}</select></div><div class="artifact-scroll">${state.viewVersion !== null && state.viewVersion !== state.baseVersion ? `<div class="notice">${icon('clock')}<div>正在查看 v${state.viewVersion}，编辑基线仍为 v${state.baseVersion}。</div>${btn('版本记录', 'versions', 'sm')}</div>` : ''}<div class="preview-meta"><span class="flex">${icon(state.tab === 'preview' ? 'eye' : 'folder')}${state.tab === 'preview' ? '已发布版本' : '版本文件与证据'} · ${esc(current().file)}</span>${state.tab === 'preview' && state.kind === 'web' ? `<div class="viewport-toggle"><button data-action="viewport" data-viewport="desktop" class="${state.viewport === 'desktop' ? 'active' : ''}" aria-label="桌面预览">${icon('desktop')}</button><button data-action="viewport" data-viewport="mobile" class="${state.viewport === 'mobile' ? 'active' : ''}" aria-label="窄屏预览">${icon('mobile')}</button></div>` : badge(current().label)}</div>${content}</div><footer class="artifact-footer"><span class="flex">${icon('shield')}隔离预览 · 按版本授权</span><span class="hide-mobile">当前内容为本地 HTML 演示</span><span>revision ${state.workspaceRevision}</span></footer></section>`; }
function workbench() { return `${header()}<nav class="mobile-tabs" aria-label="移动视图"><button data-action="mobile-tab" data-tab="chat" class="${state.mobileTab === 'chat' ? 'active' : ''}">${icon('message')}对话</button><button data-action="mobile-tab" data-tab="preview" class="${state.mobileTab === 'preview' ? 'active' : ''}">${icon('web')}作品</button><button data-action="sources">${icon('book')}来源</button></nav><div class="workbench">${thread()}${artifact()}</div>`; }
function design() { const colors = [['蓝色品牌', 'brand', '#2E6DE6'], ['绿色创作操作', 'accent', '#07C05F'], ['页面画布', 'canvas', '#F7F9FC'], ['主要文字', 'ink', '#172033'], ['次要文字', 'muted', '#66758B'], ['通用边框', 'line', '#DCE3ED'], ['成功文字', 'success-text', '#137333'], ['错误操作', 'danger', '#B42318']]; return `${header()}<section class="content-page"><div class="page-title"><div><div class="eyebrow" style="margin-bottom:6px">DESIGN SYSTEM / V1.0</div><h1>熟悉的 WeKnora，更专注的创作体验</h1><p>继承现有品牌与共享令牌；只为 Craft 补充布局、状态与组件语义。</p></div></div><div class="token-grid">${colors.map(([n, k, v]) => `<div class="swatch"><div class="swatch-color" style="background:var(--craft-${k})"></div><div class="swatch-info"><b>${n}</b><code>${v}</code><p class="tiny muted">--craft-${k}</p></div></div>`).join('')}</div><div class="section-head"><h2>组件与交互</h2>${badge('白天主题 · 已制作')}</div><div class="design-columns"><section class="design-panel"><h3>按钮层级</h3><div class="flex wrap">${btn('开始创作', 'demo-click', 'action')}${btn('导出作品', 'demo-click', 'blue')}${btn('次要操作', 'demo-click')}${btn('禁用操作', 'demo-click', '', 'disabled')}</div><p>蓝色负责品牌、导航与链接。绿色负责创作操作。亮绿按钮使用深色文字与深绿描边，避免白字对比不足；不覆盖全局 Button。</p></section><section class="design-panel"><h3>状态不只靠颜色</h3><div class="flex wrap">${badge(icon('check') + '已交付', 'good')}${badge(icon('clock') + '等待授权', 'warn')}${badge(icon('warning') + '失败', 'bad')}${badge(icon('lock') + '只读')}</div><p>图标、文案与颜色共同表达状态。连接、主任务、版本、权限和预览授权相互独立。</p></section><section class="design-panel"><h3>字体与间距</h3><div style="font-size:28px;line-height:38px;font-weight:600">让知识进入工作流</div><p>正文 14 / 22px · 辅助 12 / 20px · 标题 20 / 28px<br>间距 4 / 8 / 12 / 16 / 24 / 32 / 48px</p></section><section class="design-panel"><h3>控件与面板</h3><p>按钮圆角 6px · 卡片 8px · 弹窗 10px · 创作输入 12px<br>侧栏 224px · 对话 388px · 抽屉 460px<br>小于 760px 切换为对话 / 作品 / 来源。</p></section></div><section class="design-panel"><h3>实现交接</h3><p>颜色来源：packages/design-tokens/src/tokens.ts 与 packages/ui/src/theme.css。新增 --craft-* 采用 scoped alias，不另造全局主题。当前 HTML 是交互与视觉参考，不是已经接入 assistant-ui 或 Go 后端的生产组件。</p><div class="flex wrap" style="margin-top:16px">${btn('查看工作台', 'open-project', 'action', 'data-id="p1"')}${btn('交互状态演示', 'scenario')}</div></section></section>`; }
function drawCharts() { const css = getComputedStyle(document.body), color = n => css.getPropertyValue('--craft-' + n).trim(); $$('canvas[data-chart]').forEach(canvas => { const rect = canvas.getBoundingClientRect(); if (!rect.width)
    return; const dpr = window.devicePixelRatio || 1, w = rect.width, h = rect.height; canvas.width = w * dpr; canvas.height = h * dpr; const c = canvas.getContext('2d'); c.scale(dpr, dpr); const mini = canvas.dataset.chart === 'mini'; const left = mini ? 5 : 30, right = mini ? 5 : 8, top = mini ? 4 : 10, bottom = mini ? 6 : 22, plotH = h - top - bottom; const factor = state.region === 'east' ? .61 : state.region === 'west' ? .39 : 1; const values = seriesValues(); const previous = values.map((v, i) => v * [.87, .92, .86, .89, .88, .86][i]); c.font = `${mini ? 7 : 9}px ${css.fontFamily}`; c.textBaseline = 'middle'; if (!mini) {
    for (let t = 0; t <= 3; t++) {
        const y = top + plotH * (1 - t / 3);
        c.strokeStyle = color('line-soft');
        c.lineWidth = 1;
        c.beginPath();
        c.moveTo(left, y);
        c.lineTo(w - right, y);
        c.stroke();
        c.fillStyle = color('muted');
        c.textAlign = 'right';
        c.fillText(String(t * 10), left - 8, y);
    }
} const group = (w - left - right) / 6, bw = mini ? group * .56 : group * .23; values.forEach((v, i) => { const x = left + group * i + group * .22, y = top + plotH * (1 - v / 32); c.fillStyle = color('brand'); c.beginPath(); c.roundRect(x, y, bw, Math.max(1, plotH * v / 32), [2, 2, 0, 0]); c.fill(); if (!mini) {
    const p = previous[i];
    c.fillStyle = color('success');
    c.beginPath();
    c.roundRect(x + bw + 4, top + plotH * (1 - p / 32), bw, plotH * p / 32, [2, 2, 0, 0]);
    c.fill();
    c.fillStyle = color('muted');
    c.textAlign = 'center';
    c.fillText((i + 1) + '月', left + group * (i + .5), h - 8);
} }); }); }
function render() { const pages = { home, library, templates, workbench, design }; $('#app').innerHTML = sidebar() + `<main class="main">${(pages[state.route] || home)()}</main>`; requestAnimationFrame(drawCharts); }
function closeDrawer() { const d = $('#drawer'); if (d.open)
    d.close(); state.drawer = null; }
function openDrawer(type) { returnFocus = document.activeElement; state.drawer = type; const d = $('#drawer'); const names = { sources: ['EVIDENCE & ACCESS', '知识与资料来源'], versions: ['VERSION HISTORY', '版本记录'], upload: ['ATTACHMENTS', '添加创作资料'], choose: ['AUTHORIZED SOURCES', '选择知识范围'], scenario: ['PROTOTYPE CONTROLS', '交互状态演示'], export: ['VERSION EXPORT', '导出当前版本'] }; $('#drawer-eyebrow').textContent = names[type][0]; $('#drawer-title').textContent = names[type][1]; $('#drawer-content').innerHTML = drawerContent(type); if (!d.open)
    d.showModal(); }
function drawerContent(type) {
    if (type === 'sources')
        return `<p class="drawer-intro">当前${state.route === 'workbench' ? '查看 v' + state.viewVersion : '待提交资料'}：${esc(sourceCaption())}。下列来源目录包含可用示例；不表示每项都被当前版本引用。选择知识只影响下一次提交，历史引用不变。</p><article class="source-item"><div class="between"><h3 class="flex">${icon('table')}S1 · 销售数据.csv</h3>${badge('会话上传', 'blue')}</div><p>6 个月销售记录 · 已关联 · 示例数据</p><div class="source-excerpt">区域、月份、订单与销售额。只向执行环境提供当前任务所需的授权输入。</div>${btn('查看来源说明 ' + icon('external'), 'source-open', 'text sm', 'data-source="S1"')}</article><article class="source-item"><div class="between"><h3 class="flex">${icon('book')}S2 · 产品知识库</h3>${badge(state.sourcesRevoked ? '权限已撤销' : '已授权', state.sourcesRevoked ? 'bad' : 'good')}</div><p>团队共享知识 · 引用标识 S2</p>${state.sourcesRevoked ? '<div class="notice bad" style="margin-top:12px">无法读取受限原文。历史引用不代表永久权限。</div>' : '<div class="source-excerpt">产品分类、业务指标口径与增长策略。引用保留稳定标识，供审核时定位核对。</div>'}<div class="flex wrap">${btn('打开原文', 'source-open', 'text sm', 'data-source="S2"')}${btn(state.sourcesRevoked ? '恢复演示授权' : '模拟撤销权限', 'revoke', 'sm')}</div></article>${state.uploaded.map(f => `<article class="source-item"><h3>${icon('file')} ${esc(f.name)}</h3><p>${f.bytes} bytes · 仅本地选中，未上传后端</p></article>`).join('')}<div class="help-block">公开分享与原文访问不是同一权限。已下载文件无法仅靠撤销在线权限从接收方设备收回。</div><div class="flex wrap">${btn(icon('attach') + '添加资料', 'upload')}${btn(icon('book') + '选择知识', 'choose-sources')}</div>`;
    if (type === 'versions')
        return `<p class="drawer-intro">查看旧版不会修改工作区。恢复基线后，下一次成功交付才产生新版本。</p>${state.versions.length ? state.versions.map((v, i) => `<article class="version-item ${v.id === state.viewVersion ? 'selected' : ''}"><div class="between"><h3>v${v.id} · ${i === 0 ? '最新交付' : '历史交付'}</h3>${v.id === state.baseVersion ? badge('当前编辑基线', 'blue') : badge('独立保存')}</div><p>${v.label}<br>${v.time} · 文件 4 个</p><div class="tiny muted">恢复快照 ${v.restorable ? '完整 · 可恢复' : '不完整 · 仅可查看'}</div><div class="version-footer">${btn('查看此版本', 'view-version', 'sm', `data-version="${v.id}"`)}${btn('从此版本继续', 'restore', 'sm', `data-version="${v.id}" ${(!canWrite() || isBusy() || state.scenario === 'reconnecting' || !v.restorable) ? 'disabled' : ''}`)}</div></article>`).join('') : '<div class="empty">尚未交付第一个版本。</div>'}<div class="help-block">当前工作区 revision ${state.workspaceRevision}。活动任务、只读权限、恢复快照不完整时不能恢复。</div>`;
    if (type === 'upload')
        return `<p class="drawer-intro">正式流程复用既有上传服务，再关联到 Craft 输入。本原型只读取文件名称与大小，不读取或上传文件内容。</p><label class="upload-zone">${icon('attach')}<h3>选择用于创作的资料</h3><p>文件内容留在本地 · 支持一次选择多个文件</p><input type="file" id="local-files" multiple aria-label="选择本地资料"></label><div id="upload-results">${state.uploaded.map(f => `<article class="source-item"><b>${esc(f.name)}</b><p>${f.bytes} bytes · 本地已选择</p></article>`).join('')}</div><div class="help-block">真实文件大小、扩展名和解析能力由服务端返回；不能只靠前端后缀判断安全。上传成功也不等于知识索引已就绪。</div>${btn('保留已选资料', 'close-drawer', 'action full')}`;
    if (type === 'choose')
        return `<p class="drawer-intro">仅选择当前任务需要的材料范围。正式后端将再次检查资源访问权限。</p><div class="check-list"><label class="check-option"><input type="checkbox" value="S1" name="source-choice" ${state.selectedSources.includes('S1') ? 'checked' : ''}>${icon('table')}销售数据.csv${badge('已关联', 'blue')}</label><label class="check-option"><input type="checkbox" value="S2" name="source-choice" ${state.selectedSources.includes('S2') ? 'checked' : ''} ${state.sourcesRevoked ? 'disabled' : ''}>${icon('book')}产品知识库${badge(state.sourcesRevoked ? '已撤权' : '可检索', state.sourcesRevoked ? 'bad' : 'good')}</label></div><div class="help-block">对已经发布的版本，来源关系保持不变；此处选择影响下一次创作输入，不回写历史版本。</div>${btn('应用到下一次创作', 'apply-sources', 'action full')}`;
    if (type === 'scenario')
        return `<p class="drawer-intro">切换本地模拟状态，检查按钮、提示和恢复行为。不会触发任何真实执行。</p><div class="scenario-grid">${Object.entries(scenarioInfo).map(([k, [name, desc]]) => `<button data-action="set-scenario" data-scenario="${k}" class="${state.scenario === k ? 'active' : ''}"><b>${name}</b><span>${desc}</span></button>`).join('')}</div><div class="help-block">演示重点：查看版本 ≠ 恢复基线；断线 ≠ 任务失败；预览刷新 ≠ 重新生成；批准已记录 ≠ 已送达。</div><div class="flex wrap">${btn('模拟缺少 v1 恢复快照', 'missing-snapshot', 'sm')}${btn('重置演示', 'reset', 'sm')}</div>`;
    return `<p class="drawer-intro">当前选中 v${state.viewVersion}，不是可变工作区。原型只导出 HTML 示例，不能冒充已生成 DOCX / XLSX / PPTX。</p><article class="source-item"><h3>${esc(state.title)}</h3><p>${current().label} · 内容版本 v${state.viewVersion}</p>${btn(icon('download') + '下载 HTML 预览示例', 'download', 'blue full')}</article><article class="source-item"><h3>正式作品格式</h3><p>接入授权版本文件 API 后启用对应格式。网页打包下载需要独立打包契约。</p>${btn('正式文件下载尚未接入', 'demo-click', 'full', 'disabled')}</article><div class="help-block">本地导出的页面会标注“原型示例”，不携带模型凭据、原文附件或后端预览票据。</div>`;
}
function confirmation(title, text, action, label, attrs = '') { returnFocus = document.activeElement; const d = $('#confirm'); $('#confirm-content').innerHTML = `<h2 id="confirm-title">${esc(title)}</h2><p>${text}</p><div class="modal-actions">${btn('取消', 'close-confirm')}${btn(label, action, 'action', attrs)}</div>`; if (!d.open)
    d.showModal(); }
function openProject(id) { const p = projects.find(p => p.id === id) || projects[0]; state.kind = p.kind; state.title = p.title; state.viewVersion = p.version; state.baseVersion = p.version; state.versions = Array.from({ length: p.version }, (_, i) => ({ id: p.version - i, label: i === 0 ? '完善内容与来源说明' : i === 1 ? '补充结构与业务指标' : '生成第一版作品', time: i === 0 ? '今天 10:42' : '今天 10:16', restorable: true, sourceIds: ['S1', 'S2'] })); state.scenario = 'ready'; state.workspaceRevision = 7; state.runCount = 1; state.tab = 'preview'; state.draft = ''; state.mobileTab = 'chat'; state.slide = 1; state.lastPrompt = p.kind === 'web' ? '结合销售数据和产品知识，生成季度经营分析看板。需要汇总指标、区域筛选，以及可追溯的业务结论。' : `根据我选择的团队资料，生成${p.title}。内容清晰，保留来源，并支持继续修改。`; go('workbench'); }
function create() { if (!state.homeDraft.trim()) {
    showToast('请先描述要创作的内容。');
    $('#home-prompt').focus();
    return;
} state.activeSourceIds = [...state.selectedSources]; state.lastPrompt = state.homeDraft.trim(); state.title = current().name; state.versions = []; state.viewVersion = null; state.baseVersion = null; state.workspaceRevision = 1; state.runCount = 1; state.scenario = 'running'; state.tab = 'preview'; state.mobileTab = 'chat'; state.draft = ''; go('workbench'); }
function send() { if (!canSend()) {
    showToast('当前不能提交新任务。草稿已保留。');
    return;
} if (!state.draft.trim()) {
    showToast('请先输入修改要求。');
    $('#composer')?.focus();
    return;
} state.activeSourceIds = [...state.selectedSources]; state.lastPrompt = state.draft.trim(); state.draft = ''; state.runCount++; state.scenario = 'running'; render(); }
function finish() { if (state.scenario !== 'running') {
    showToast('仅正在执行的演示任务可以完成。');
    return;
} const next = Math.max(0, ...state.versions.map(v => v.id)) + 1; state.versions.unshift({ id: next, label: '根据本轮目标修改并交付（模拟）', time: '本次演示', restorable: true, sourceIds: [...(state.activeSourceIds || state.selectedSources)] }); state.viewVersion = next; state.baseVersion = next; state.workspaceRevision++; state.scenario = 'ready'; render(); showToast(`已模拟发布 v${next}，历史版本保持不变。`); }
function download() { closeDrawer(); const text = { web: report, document: documentPreview, spreadsheet: spreadsheetPreview, slides: slidesPreview }[state.kind](); const styles = Array.from(document.querySelectorAll('style')).map(s => s.textContent).join('\n'); const data = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${esc(state.title)} · v${state.viewVersion} · 原型示例</title><style>${styles}\nbody{padding:30px;max-width:1100px;margin:auto}</style><body class="wk-craft"><p style="margin-bottom:20px">原型导出示例 · 模拟数据 · 版本 v${state.viewVersion} · 非真实业务交付文件</p>${text}${state.kind === 'web' ? '<p style="margin-top:16px">此离线示例不包含交互脚本；图表以表格数据补充。</p><table class="data-table">' : ''}${state.kind === 'web' ? '<tr><th>月份</th><th>示例销售 / 万元</th></tr>' + seriesValues().map((v, i) => `<tr><td>${i + 1}月</td><td>${v.toFixed(2)}</td></tr>`).join('') + '</table>' : ''}</body></html>`; const b = new Blob([data], { type: 'text/html;charset=utf-8' }), u = URL.createObjectURL(b), a = document.createElement('a'); a.href = u; a.download = `craft-preview-v${state.viewVersion}-demo.html`; a.click(); setTimeout(() => URL.revokeObjectURL(u), 2000); showToast(`已下载 v${state.viewVersion} 的 HTML 示例，不是正式 Office 文件。`); }
document.addEventListener('click', e => { const r = e.target.closest('[data-route]'); if (r) {
    e.preventDefault();
    go(r.dataset.route);
    return;
} const el = e.target.closest('[data-action]'); if (!el || el.disabled)
    return; e.preventDefault(); const a = el.dataset.action; switch (a) {
    case 'new':
        state.homeDraft = '';
        go('home');
        $('#home-prompt')?.focus();
        break;
    case 'library':
        go('library');
        break;
    case 'templates':
        go('templates');
        break;
    case 'kind':
        state.kind = el.dataset.kind;
        render();
        break;
    case 'template':
        state.kind = el.dataset.kind;
        state.homeDraft = `根据我选择的团队资料，生成${current().name}。保留关键数据与来源，支持继续修改。`;
        go('home');
        $('#home-prompt')?.focus();
        break;
    case 'create':
        create();
        break;
    case 'open-project':
        openProject(el.dataset.id);
        break;
    case 'sources':
        openDrawer('sources');
        break;
    case 'versions':
        openDrawer('versions');
        break;
    case 'upload':
        openDrawer('upload');
        break;
    case 'choose-sources':
        openDrawer('choose');
        break;
    case 'scenario':
        openDrawer('scenario');
        break;
    case 'export':
        if (state.viewVersion !== null)
            openDrawer('export');
        break;
    case 'close-drawer':
        closeDrawer();
        render();
        break;
    case 'close-confirm':
        $('#confirm').close();
        break;
    case 'filter':
        state.filter = el.dataset.filter;
        render();
        break;
    case 'clear-filter':
        state.filter = 'all';
        state.search = '';
        render();
        break;
    case 'tab':
        state.tab = el.dataset.tab;
        render();
        break;
    case 'checks':
        state.tab = 'checks';
        state.mobileTab = 'preview';
        render();
        break;
    case 'preview':
        state.tab = 'preview';
        state.mobileTab = 'preview';
        render();
        break;
    case 'viewport':
        state.viewport = el.dataset.viewport;
        render();
        break;
    case 'mobile-tab':
        state.mobileTab = el.dataset.tab;
        render();
        break;
    case 'suggestion':
        state.draft = el.dataset.text;
        render();
        $('#composer')?.focus();
        break;
    case 'send':
        send();
        break;
    case 'finish':
        finish();
        break;
    case 'approve':
        state.decision = 'recorded';
        render();
        break;
    case 'ack':
        if (state.decision === 'recorded') {
            state.decision = 'delivered';
            state.scenario = 'running';
            render();
            showToast('模拟已确认送达，继续同一任务。');
        }
        break;
    case 'reject':
        state.scenario = 'failed';
        state.decision = 'rejected';
        render();
        showToast('已拒绝本次请求，没有授予权限。');
        break;
    case 'answer':
        state.scenario = 'running';
        render();
        showToast('已模拟回答问题，不增加执行权限。');
        break;
    case 'stop':
        confirmation('停止当前任务？', '将请求后端取消，并核对执行状态。关闭浏览器或断开连接不等于取消任务。', 'confirm-stop', '确认停止');
        break;
    case 'confirm-stop':
        $('#confirm').close();
        state.scenario = 'canceled';
        render();
        showToast('模拟任务已取消；上一交付版本保留。');
        break;
    case 'reconnect':
        state.scenario = 'ready';
        render();
        showToast('已同步原任务，未创建新的 Run。');
        break;
    case 'refresh-preview':
        state.scenario = 'ready';
        render();
        showToast('仅续期预览授权，没有重新生成作品。');
        break;
    case 'view-version':
        state.viewVersion = Number(el.dataset.version);
        state.tab = 'preview';
        closeDrawer();
        render();
        showToast(`查看 v${state.viewVersion}，编辑基线仍是 v${state.baseVersion}。`);
        break;
    case 'restore':
        state.restoreTarget = Number(el.dataset.version);
        closeDrawer();
        confirmation(`从 v${state.restoreTarget} 继续编辑？`, '将检查写权限、活动任务和恢复快照，恢复工作区并提升 revision。不会覆盖历史版本，也不会仅因恢复就生成新版本。', 'confirm-restore', '恢复编辑基线');
        break;
    case 'confirm-restore': {
        const v = state.versions.find(v => v.id === state.restoreTarget);
        if (!v || !v.restorable || !canWrite() || isBusy() || state.scenario === 'reconnecting') {
            showToast('当前条件不允许恢复。');
            $('#confirm').close();
            break;
        }
        state.baseVersion = v.id;
        state.viewVersion = v.id;
        state.workspaceRevision++;
        $('#confirm').close();
        render();
        showToast(`已模拟恢复到 v${v.id}，历史版本数量未变化。`);
        break;
    }
    case 'apply-sources':
        state.selectedSources = $$('input[name="source-choice"]:checked:not(:disabled)').map(x => x.value);
        closeDrawer();
        render();
        showToast('已更新下一次创作的资料范围。');
        break;
    case 'revoke':
        state.sourcesRevoked = !state.sourcesRevoked;
        openDrawer('sources');
        break;
    case 'source-open':
        if (el.dataset.source === 'S2' && state.sourcesRevoked)
            showToast('403 演示：来源权限已撤销，不能继续读取。');
        else
            confirmation('来源说明 · ' + el.dataset.source, '这是授权资料定位的演示。正式开发按稳定 citation ID 调用已有资源入口并重新检查权限；本原型不含真实企业原文。', 'close-confirm', '知道了');
        break;
    case 'file':
        confirmation('版本文件元信息', `路径：<code>${esc(el.dataset.file)}</code><br>版本：v${state.viewVersion}<br>SHA-256：原型没有真实版本文件，不伪造校验值。<br>正式实现从 manifest 获取元信息。`, 'close-confirm', '知道了');
        break;
    case 'set-scenario':
        state.scenario = el.dataset.scenario;
        state.decision = 'pending';
        closeDrawer();
        if (state.route !== 'workbench')
            go('workbench');
        else
            render();
        break;
    case 'missing-snapshot': {
        const v = state.versions.find(v => v.id === 1);
        if (v)
            v.restorable = false;
        closeDrawer();
        openDrawer('versions');
        break;
    }
    case 'reset':
        closeDrawer();
        location.href = location.pathname + '#home';
        location.reload();
        break;
    case 'download':
        download();
        break;
    case 'slide':
        state.slide = Number(el.dataset.slide);
        render();
        break;
    case 'prev-slide':
        state.slide = Math.max(1, state.slide - 1);
        render();
        break;
    case 'next-slide':
        state.slide = Math.min(4, state.slide + 1);
        render();
        break;
    case 'menu':
        document.body.classList.toggle('nav-open');
        break;
    case 'close-nav':
        document.body.classList.remove('nav-open');
        break;
    case 'space':
        confirmation('产品团队工作空间', '此处复用现有 WeKnora 空间切换。原型仅有一个演示空间，不会创建新的租户或登录体系。', 'close-confirm', '知道了');
        break;
    case 'demo-click':
        showToast('组件状态演示；没有发送网络请求。');
        break;
} });
document.addEventListener('input', e => { if (e.target.id === 'home-prompt')
    state.homeDraft = e.target.value; if (e.target.id === 'composer')
    state.draft = e.target.value; if (e.target.id === 'project-search') {
    state.search = e.target.value;
    $('#project-grid').innerHTML = projectResults();
    requestAnimationFrame(drawCharts);
} });
document.addEventListener('change', e => { if (e.target.id === 'version-select') {
    state.viewVersion = Number(e.target.value);
    render();
} if (e.target.id === 'region') {
    state.region = e.target.value;
    render();
} if (e.target.id === 'local-files') {
    state.uploaded = Array.from(e.target.files || []).map(f => ({ name: f.name, bytes: f.size }));
    $('#upload-results').innerHTML = state.uploaded.map(f => `<article class="source-item"><h3>${esc(f.name)}</h3><p>${f.bytes} bytes · 本地已选择，未上传</p></article>`).join('');
} });
document.addEventListener('keydown', e => { if (e.target.id === 'composer' && e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault();
    send();
} if (e.key === 'Escape')
    document.body.classList.remove('nav-open'); });
$('#drawer').addEventListener('close', () => { state.drawer = null; if (returnFocus?.isConnected)
    returnFocus.focus(); });
$('#confirm').addEventListener('close', () => { if (returnFocus?.isConnected)
    returnFocus.focus(); });
window.addEventListener('popstate', () => { state.route = location.hash.slice(1) || 'home'; render(); });
window.addEventListener('resize', () => requestAnimationFrame(drawCharts));
window.CraftDemo = { snapshot: () => JSON.parse(JSON.stringify(state)) };
const initial = location.hash.slice(1);
if (['home', 'library', 'templates', 'workbench', 'design'].includes(initial))
    state.route = initial;
render();
