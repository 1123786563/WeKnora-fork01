#!/usr/bin/env node
/**
 * Phase I 面板矩阵生成：读 panel-matrix/inventory.json 产出 matrix.md 草稿
 * （页面×触发器×面板类型×双端一致性×建议 + 统计 + 分批建议）。
 * 用法：node scripts/parity/panel-matrix-gen.mjs
 */
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = '/Users/wuyongjun/trea/WeKnora-fork01';
const OUT = join(ROOT, 'docs/migrations/react/evidence/vue-react-parity/panel-matrix');
const inv = JSON.parse(readFileSync(join(OUT, 'inventory.json'), 'utf8'));

// ---- 触发器归一键（双端 join 用） ----
function tkey(c) {
  return (c.guid ? 'g:' + c.guid : c.testid ? 't:' + c.testid : c.aria ? 'a:' + c.aria
    : c.title ? 'ti:' + c.title : c.text ? 'x:' + c.text : 'c:' + String(c.cls).split('.').slice(0, 2).join('.')).toLowerCase();
}
// ---- 选择器兜底链 ----
function chain(c) {
  const parts = [];
  if (c.guid) parts.push(`[data-guide="${c.guid}"]`);
  if (c.testid) parts.push(`[data-testid="${c.testid}"]`);
  if (c.aria) parts.push(`[aria-label*="${c.aria}"]`);
  else if (c.title) parts.push(`[title*="${c.title}"]`);
  if (c.cls && String(c.cls)) {
    const compound = '.' + String(c.cls).split('.').filter(Boolean).slice(0, 2).join('.');
    if (compound !== '.') parts.push(compound);
  }
  if (c.text) parts.push(`text:"${c.text}"`);
  return parts.join(' > ');
}
function label(c) {
  return c.aria || c.title || c.text || (c.cls ? '.' + String(c.cls).split('.')[0] : c.tag);
}
function typesOf(c) { return c.panel ? c.panel.types.filter(t => t !== 'message') : []; }

const DOMAIN = (id) => {
  if (id === 'kb-list' || id.startsWith('kb-')) return '知识库';
  if (id === 'chat') return '对话';
  if (['agents', 'orgs', 'creatchat', 'apps', 'apps-connections'].includes(id)) return '平台路由';
  if (id.startsWith('settings-integration-')) return '设置-集成';
  if (id.startsWith('settings-')) return '设置';
  if (['login', 'register'].includes(id)) return '免登录';
  return '其他';
};

// ---- 汇总 ----
const rows = [];
const hetero = [];
let stat = { triggers: 0, panelTriggers: 0, suggestNew: 0, ixReuse: 0, denyOnly: 0 };
for (const page of inv.results) {
  const ends = page.ends;
  const vue = ends.vue && !ends.vue.clonedFrom ? ends.vue.candidates || [] : null;
  const react = ends.react && !ends.react.clonedFrom ? ends.react.candidates || [] : null;
  const cloneNote = ends.vue?.clonedFrom || ends.react?.clonedFrom;
  const map = new Map();
  for (const c of vue || []) { const k = tkey(c); if (!map.has(k)) map.set(k, { vue: null, react: null }); map.get(k).vue = c; }
  for (const c of react || []) { const k = tkey(c); if (!map.has(k)) map.set(k, { vue: null, react: null }); map.get(k).react = c; }
  for (const [k, { vue: v, react: r }] of map) {
    const pick = v || r;
    const tv = typesOf(v || {}).join('+');
    const tr = typesOf(r || {}).join('+');
    const hasPanel = !!(tv || tr);
    const both = v && r;
    const same = both && tv === tr && hasPanel;
    const consist = both ? (hasPanel ? (tv === tr ? '一致' : '类型异') : '一致(无面板)') : (pick.ix || ['nav-link', 'deny'].includes(pick.category) ? '单端枚举' : '异构');
    let advice, batch = '';
    if (pick.ix) { advice = `复用 ${pick.ix}`; stat.ixReuse++; }
    else if (hasPanel && both && tv === tr) { advice = `新增扫描项：${tv}`; stat.suggestNew++; batch = 'P2'; }
    else if (hasPanel && both && tv !== tr) { advice = `异构面板(${tv} vs ${tr})→核实后新增`; stat.suggestNew++; batch = 'P2'; hetero.push(`${page.id}/${label(pick)}: ${tv} vs ${tr}`); }
    else if (hasPanel && !both) { advice = `单端面板(${v ? 'Vue' : 'React'}:${v ? tv : tr})→核实对端缺失`; hetero.push(`${page.id}/${label(pick)}: 仅${v ? 'Vue' : 'React'} ${v ? tv : tr}`); batch = 'P2'; }
    else if (!hasPanel && pick.category === 'deny') { advice = `排除：${(pick.reason || 'deny').replace(/^deny:/, '')}类动作（只记录）`; stat.denyOnly++; }
    else if (pick.category === 'nav-link' || pick.category === 'nav-jump') advice = '排除：导航跳转';
    else if (pick.category === 'inline-tab') advice = '排除：tab 内联切换';
    else if (pick.category === 'toggle') advice = '排除：开关';
    else if (pick.category === 'pagination') advice = '排除：分页';
    else advice = '排除：无面板';
    stat.triggers++;
    if (hasPanel) stat.panelTriggers++;
    const shot = (v && v.shot) || (r && r.shot) || '';
    rows.push({ page: page.id, domain: DOMAIN(page.id), k, label: label(pick), chain: chain(pick), tv, tr, consist, advice, shot, cloneNote, hover: pick.hover });
  }
}

// ---- 分批建议 ----
const newItems = rows.filter(r2 => r2.advice.startsWith('新增扫描项'));
const domains = [...new Set(newItems.map(r2 => r2.domain))];
const batches = [];
const domainOrder = ['知识库', '对话', '平台路由', '设置', '设置-集成', '免登录', '其他'];
let cur = [];
for (const d of domainOrder) {
  const items = newItems.filter(r2 => r2.domain === d);
  if (!items.length) continue;
  if (cur.length && cur.length + items.length > 20) { batches.push(cur); cur = []; }
  cur.push(...items);
}
if (cur.length) batches.push(cur);

// ---- 输出 ----
const L = [];
L.push('# 交互面板盘点矩阵（Phase I）', '', `- 生成：${inv.stamp}`, `- 双端：Vue ${inv.vue} / React ${inv.react}`,
  '- 数据：panel-matrix/inventory.json（scripts/parity/panel-inventory.mjs 产出）',
  '- 方法：headless 真实鼠标点击逐触发器打开面板（破坏性/写操作类只记录不实点），浮层基线 diff + aria-expanded 兜底检测，ESC/空白点击/重载关闭；截图 png/。', '');

L.push('## 总表（按页面域分组）', '');
let lastDomain = '';
for (const r2 of rows.sort((a, b) => domainOrder.indexOf(a.domain) - domainOrder.indexOf(b.domain) || a.page.localeCompare(b.page))) {
  if (r2.domain !== lastDomain) { L.push('', `### ${r2.domain}`, '', '| 页面 | 触发器 | 选择器兜底链 | 面板(Vue) | 面板(React) | 双端 | 建议 | 截图 |', '|---|---|---|---|---|---|---|---|'); lastDomain = r2.domain; }
  L.push(`| ${r2.page}${r2.cloneNote ? '（同 ' + r2.cloneNote + '）' : ''} | ${r2.hover ? '⇱ ' : ''}${r2.label} | \`${r2.chain}\` | ${r2.tv || '-'} | ${r2.tr || '-'} | ${r2.consist} | ${r2.advice} | ${r2.shot ? r2.shot : ''} |`);
}
L.push('', '## 统计', '',
  `- 触发器总数（页面×归一触发器）：${stat.triggers}`,
  `- 打开面板的触发器：${stat.panelTriggers}`,
  `- 建议新增扫描项：${stat.suggestNew}`,
  `- 复用既有 ix-* 项：${stat.ixReuse}`,
  `- deny 只记录（破坏性/写操作动词）：${stat.denyOnly}`,
  `- 双端异构点：${hetero.length}`, '');
if (hetero.length) { L.push('### 双端异构点清单', ''); for (const h of hetero) L.push(`- ${h}`); }
L.push('', '## Phase II 分批建议', '');
batches.forEach((b, i) => {
  L.push(`### 批 ${i + 1}（${b.length} 项：${[...new Set(b.map(x => x.domain))].join('/')}）`, '');
  for (const x of b) L.push(`- ${x.page} — ${x.label}（${x.tv === x.tr ? x.tv : x.tv + ' vs ' + x.tr}）选择器：\`${x.chain}\``);
  L.push('');
});
writeFileSync(join(OUT, 'matrix-draft.md'), L.join('\n') + '\n');
console.log(`[draft] ${join(OUT, 'matrix-draft.md')} | triggers=${stat.triggers} panels=${stat.panelTriggers} suggest=${stat.suggestNew} ix=${stat.ixReuse} hetero=${hetero.length} batches=${batches.length}`);
