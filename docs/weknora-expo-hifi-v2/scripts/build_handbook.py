"""Optional HTML reader for the Markdown handoff. Requires markdown-it-py."""
from pathlib import Path
from markdown_it import MarkdownIt
import html,json,re,os
R=Path(__file__).resolve().parent.parent
paths=[R/'README.md',*sorted((R/'docs').glob('*.md')),*sorted((R/'plans').glob('*.md'))]
md=MarkdownIt('commonmark',{'html':False}).enable('table')
lookup={p.resolve():i for i,p in enumerate(paths)}
sections=[];nav=[]
for i,p in enumerate(paths):
 source=p.read_text();title=source.splitlines()[0].lstrip('# ');text=md.render(source)
 def fix(m):
  href=html.unescape(m.group(1))
  if href.startswith(('http:','https:','#','mailto:')):return m.group(0)
  raw=href.split('#')[0];target=(p.parent/raw).resolve()
  if target in lookup:return f'href="#doc-{lookup[target]}"'
  return 'href="'+html.escape(os.path.relpath(target,R),quote=True)+'"'
 text=re.sub(r'href="([^"]+)"',fix,text)
 sections.append(f'<article id="doc-{i}" data-title="{html.escape(title)}" {"" if i==0 else "hidden"}>{text}</article>')
 nav.append(f'<a href="#doc-{i}" data-nav="{i}">{html.escape(title)}</a>')
page='''<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>WeKnora 开发交付手册</title><style>*{box-sizing:border-box}body{margin:0;background:#f5f7f2;color:#1b302b;font-family:system-ui,-apple-system,'PingFang SC','Noto Sans CJK SC',sans-serif;line-height:1.85}header{background:white;border-bottom:1px solid #dfe6de;padding:19px 28px;display:flex;justify-content:space-between;gap:20px;align-items:center;position:sticky;top:0;z-index:3}header b{font-size:21px}header a{font-size:14px;color:#08766a}aside{position:fixed;left:0;top:81px;width:275px;padding:23px 15px;height:calc(100vh - 81px);overflow:auto;border-right:1px solid #dfe6de}aside a{display:block;color:#60716b;text-decoration:none;padding:9px 12px;font-size:13px;border-radius:10px;line-height:1.6;margin-bottom:5px}aside a.active{background:#e1f1e9;color:#065d54;font-weight:650}main{margin-left:275px;max-width:1260px;padding:28px 40px 80px}article{background:#fff;border:1px solid #dfe6de;border-radius:18px;padding:36px;overflow-wrap:anywhere}h1{font-size:29px;line-height:1.5;letter-spacing:-.6px}h2{font-size:23px;margin-top:38px;border-bottom:1px solid #dfe6de;padding-bottom:8px}h3{font-size:18px;margin-top:25px}p,li{font-size:15px}a{color:#08766a}table{border-collapse:collapse;display:block;max-width:100%;overflow:auto;font-size:13px;margin:22px 0}td,th{border:1px solid #dfe6de;padding:9px 12px;text-align:left;vertical-align:top;min-width:100px}th{background:#eef2ed}code{font-family:ui-monospace,Consolas,monospace;font-size:.88em;background:#eef2ed;padding:2px 5px;border-radius:4px}pre{padding:18px;background:#11251d;color:#eaf4ef;overflow:auto;border-radius:12px;font-size:13px;line-height:1.8}pre code{background:none;padding:0;color:inherit}blockquote{border-left:3px solid #08766a;margin:20px 0;padding:5px 18px;background:#f6f7f3}img{max-width:100%}[hidden]{display:none!important}@media(max-width:800px){header{padding:14px 17px;position:static;flex-wrap:wrap}header b{font-size:18px}aside{position:static;width:auto;height:auto;display:flex;overflow:auto;border-right:0;padding:9px;gap:4px}aside a{min-width:145px;max-width:230px;flex-shrink:0;margin:0}main{margin:0;padding:10px}article{padding:19px}h1{font-size:24px}h2{font-size:21px}}</style><header><b>WeKnora / 开发交付手册</b><span><a href="index.html">高保真原型</a>　<a href="design-system.html">设计系统</a></span></header><aside>__NAV__</aside><main>__DOCS__</main><script>function openDoc(){let id=location.hash||'#doc-0';if(!document.querySelector(id))id='#doc-0';document.querySelectorAll('article').forEach(e=>e.hidden='#'+e.id!==id);document.querySelectorAll('[data-nav]').forEach(e=>e.classList.toggle('active',e.getAttribute('href')===id));window.scrollTo(0,0)}window.addEventListener('hashchange',openDoc);openDoc();</script></html>'''
(R/'handoff.html').write_text(page.replace('__NAV__',''.join(nav)).replace('__DOCS__',''.join(sections)))
print('Built handoff.html with',len(paths),'document sections.')
