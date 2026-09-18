"""Python stdlib only. Resolve token aliases and generate logical CSS / Taro SCSS / TS."""
from pathlib import Path
import json,re
R=Path(__file__).resolve().parent
obj=json.loads((R/'tokens.json').read_text());ts=obj['tokens']
def resolve(k,trail=()):
 if k in trail:raise ValueError('Circular token alias: '+k)
 v=ts[k]['$value']
 return resolve(v[1:-1],trail+(k,)) if isinstance(v,str) and re.fullmatch(r'\{[^{}]+\}',v) else v
def name(k):return re.sub(r'([a-z0-9])([A-Z])',r'\1-\2',k).replace('.','-').lower()
def value(k,taro=False):
 v=resolve(k);t=ts[k]['$type']
 if t=='dimension':return f'{v*2 if taro else v}px'
 if t=='fixedDimension':return f'{v}{"Px" if taro else "px"}'
 if t=='duration':return str(v)+'ms'
 if t=='shadow' and taro:return re.sub(r'(-?[0-9.]+)px',lambda m:f'{float(m[1])*2:g}px',v)
 return str(v)
(R/'tokens.css').write_text('/* GENERATED. Logical 375px units; do not hand edit. */\n:root{\n'+''.join(f'--{name(k)}:{value(k)};\n' for k in ts)+'}\n')
(R/'_tokens.taro.scss').write_text('// GENERATED. designWidth=750: logical dimensions x2. Runtime metrics NOT doubled.\n'+''.join(f'${name(k)}: {value(k,True)};\n' for k in ts))
resolved={k:resolve(k) for k in ts}
(R/'tokens.ts').write_text('// GENERATED. Logical dimensions; not preconverted rpx.\nexport const tokens = '+json.dumps(resolved,ensure_ascii=False,indent=2)+' as const;\nexport type TokenName = keyof typeof tokens;\n')
(R/'resolved-tokens.json').write_text(json.dumps(resolved,ensure_ascii=False,indent=2)+'\n')
print('Generated CSS, SCSS, TS from',len(ts),'tokens')
