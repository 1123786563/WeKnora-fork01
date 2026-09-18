from pathlib import Path
import json,re
R=Path(__file__).resolve().parents[1]
def lum(h):
 c=[int(h.lstrip('#')[i:i+2],16)/255 for i in (0,2,4)]
 c=[v/12.92 if v<=.04045 else ((v+.055)/1.055)**2.4 for v in c]
 return .2126*c[0]+.7152*c[1]+.0722*c[2]
def ratio(a,b):
 x,y=sorted([lum(a),lum(b)],reverse=True)
 return (x+.05)/(y+.05)
pairs=[('blue action / white text','#ffffff','#2e6de6',True),('green action / dark ink','#172033','#07c05f',True),('green hover / dark ink','#172033','#08dd6e',True),('green active / dark ink','#172033','#06b04d',True),('body / white','#172033','#ffffff',True),('muted / white','#66758b','#ffffff',True),('accessible secondary / canvas','#506078','#f7f9fc',True),('accessible secondary / sidebar','#506078','#f9f9f9',True),('original muted / canvas (NOT USED for secondary copy)','#66758b','#f7f9fc',False),('success text / soft green','#137333','#e9f8ec',True),('warning text / wash','#9a6700','#fffaeb',True),('danger text / wash','#b42318','#fdecea',True),('legacy bright green / white text (NOT USED for Craft actions)','#ffffff','#07c05f',False)]
rows=[{'pair':n,'foreground':a,'background':b,'ratio':round(ratio(a,b),3),'normal_text_target':4.5,'meets_4_5':ratio(a,b)>=4.5,'used_as_action_text_or_body':used} for n,a,b,used in pairs]
assert all(r['meets_4_5'] for r in rows if r['used_as_action_text_or_body']),rows
css=(R/'design/tokens.css').read_text();html=(R/'index.html').read_text();tasks=json.loads((R/'docs/tasks.json').read_text())['tasks']
assert len(tasks)==36 and len({t['id'] for t in tasks})==36
assert all((R/'docs/tasks'/f"{t['id']}.md").exists() for t in tasks)
byid={t['id']:t for t in tasks}
assert all(byid[d]['number']<t['number'] for t in tasks for d in t['dependencies'])
assert '#2e6de6' in css and '#07c05f' in css and '#235e50' not in css.lower()
assert not re.search(r'<(?:script|link)[^>]+(?:src|href)=["\']https?://',html,re.I)
assert not list(R.rglob('*.woff*')) and not list(R.rglob('*.ttf')) and not list(R.rglob('*.otf'))
assert len(re.findall(r'--craft-[a-z0-9-]+\s*:',css))>=60
json.loads((R/'design/tokens.json').read_text())
(R/'qa/contrast-results.json').write_text(json.dumps({'method':'WCAG relative luminance (sRGB), no opacity/gradient in these pairs','scope':'core solid-color pairs; not a complete accessibility audit','pairs':rows},ensure_ascii=False,indent=2))
(R/'qa/design-results.json').write_text(json.dumps({'status':'passed','tasks':36,'dependency_graph':'all ids resolve and DAG valid','token_declarations':len(re.findall(r'--craft-[a-z0-9-]+\s*:',css)),'remote_cdn':'none','font_files_included':0,'scope':'artifact consistency only; no business repository compilation'},ensure_ascii=False,indent=2))
print('Artifact consistency passed; contrast',[(r['pair'],r['ratio']) for r in rows])
