"""Validate this design handoff; this is NOT a WeKnora/Expo production test."""
from pathlib import Path
import json,re,itertools,hashlib
R=Path(__file__).resolve().parent.parent
errors=[];checks=[]
def check(name, condition):
 checks.append({'name':name,'pass':bool(condition)})
 if not condition:errors.append(name)
P=json.loads((R/'fixtures/pages.json').read_text());T=json.loads((R/'plans/task-index.json').read_text())['tasks'];ids={t['id'] for t in T}
check('18 unique page IDs',len(P)==len({p['no'] for p in P})==18)
check('36 unique task IDs',len(T)==len(ids)==36)
check('all pages mapped',all(p['task'] in ids for p in P))
check('all tasks pending',all(t['status']=='pending' for t in T))
check('G01-G12 mapped',all(any(f'G{i:02}' in t['closes_gaps'] for t in T) for i in range(1,13)))
profiles=sorted({k for t in T for k in t['conditional_dependencies']})
for flags in itertools.product([False,True],repeat=len(profiles)):
 active={k for k,v in zip(profiles,flags) if v};edges={t['id']:set(t['depends_on'])|{d for p,ds in t['conditional_dependencies'].items() if p in active for d in ds} for t in T}
 valid=all(ds<=ids for ds in edges.values());done=set()
 while valid:
  ready={k for k,ds in edges.items() if k not in done and ds<=done}
  if not ready:break
  done|=ready
 check('DAG '+(','.join(sorted(active)) or 'core'),valid and len(done)==36)
for p in P:check('standalone '+p['no'],(R/'pages'/f'{p["no"]}-{p["id"]}.html').exists())
check('no shared fonts',not any(p.suffix.lower() in ['.ttf','.otf','.woff','.woff2','.ttc'] for p in R.rglob('*')))
# Markdown links in new authored docs/plans; references are preserved historical source artifacts.
broken=[]
for p in [* (R/'docs').glob('*.md'),*(R/'plans').glob('*.md')]:
 for href in re.findall(r'\]\(([^)]+)\)',p.read_text()):
  if href.startswith(('http:','https:','#','mailto:')):continue
  target=href.split('#')[0]
  if target and not (p.parent/target).exists():broken.append({'file':str(p.relative_to(R)),'target':target})
check('authored local markdown links',not broken)
x=json.loads((R/'tokens/tokens.json').read_text());light=x['theme']['light'];dark=x['theme']['dark']
check('theme key parity',set(light)==set(dark))
def lum(c):
 rgb=[int(c[i:i+2],16)/255 for i in (1,3,5)];v=[r/12.92 if r<=.04045 else ((r+.055)/1.055)**2.4 for r in rgb]
 return sum(a*b for a,b in zip(v,[.2126,.7152,.0722]))
def contrast(a,b):
 aa,bb=sorted([lum(a),lum(b)]);return (bb+.05)/(aa+.05)
contrast_report=[]
pairs=[('ink','surface',4.5),('ink','bg',4.5),('muted','surface',4.5),('muted','bg',4.5),('subtle','surface',4.5),('on-brand','brand',4.5),('brand','brand-soft',4.5),('warning','warning-soft',4.5),('danger','danger-soft',4.5),('info','info-soft',4.5),('purple','purple-soft',4.5),('hero-ink','hero',4.5),('control-line','surface',3)]
for mode,group in x['theme'].items():
 for fg,bg,threshold in pairs:
  r=contrast(group[fg]['$value'],group[bg]['$value']);contrast_report.append({'theme':mode,'foreground':fg,'background':bg,'ratio':r,'minimum':threshold,'pass':r>=threshold});check(f'contrast {mode} {fg}/{bg}',r>=threshold)
check('proposal not production labeled','Design proposal only' in (R/'contracts/mobile-proposal.ts').read_text())
report={'scope':'Generated artifact structure, task DAG and selected token pairs only; no production/native validation','checks':checks,'passed':sum(c['pass'] for c in checks),'failed':len(errors),'errors':errors,'broken_links':broken,'profile_combinations':2**len(profiles)}
(R/'verification/static-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
(R/'verification/token-contrast.json').write_text(json.dumps({'method':'sRGB relative luminance; threshold compared without rounding','not_a_full_wcag_audit':True,'pairs':contrast_report},ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'passed':report['passed'],'failed':len(errors),'errors':errors,'broken_links':broken,'profile_combinations':report['profile_combinations']},ensure_ascii=False,indent=2))
raise SystemExit(1 if errors else 0)
