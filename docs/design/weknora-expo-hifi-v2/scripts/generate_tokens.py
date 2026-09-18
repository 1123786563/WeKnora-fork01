"""Generate web/native token outputs from tokens.json. No third-party libraries."""
from pathlib import Path
import json
R=Path(__file__).resolve().parent.parent
tokens=json.loads((R/'tokens/tokens.json').read_text())
colors={mode:{k:v['$value'] for k,v in group.items()} for mode,group in tokens['theme'].items()}
css='/* Generated from tokens.json. Do not edit individual page copies. */\n'
for mode in ['light','dark']:
 css+=(':root, [data-theme="light"]' if mode=='light' else '[data-theme="dark"]')+'{\n'+''.join(f'  --{k}: {v};\n' for k,v in colors[mode].items())+'}\n'
css+=':root{\n'+''.join(f'  --space-{k}: {v["$value"]}px;\n' for k,v in tokens['space'].items())+''.join(f'  --radius-{k}: {v["$value"]}px;\n' for k,v in tokens['radius'].items())+''.join(f'  --font-{k}: {v["fontSize"]}px; --line-{k}: {v["lineHeight"]}px; --weight-{k}: {v["fontWeight"]};\n' for k,v in tokens['typography'].items())+''.join(f'  --{k}: {v}px;\n' for k,v in tokens['size'].items())+''.join(f'  --motion-{k}: {v}ms;\n' for k,v in tokens['motion'].items())+'}\n'
(R/'tokens/tokens.css').write_text(css)
native={'colors':colors,'spacing':{k:v['$value'] for k,v in tokens['space'].items()},'radius':{k:v['$value'] for k,v in tokens['radius'].items()},'typography':{k:{**v,'fontWeight':str(600 if v['fontWeight']==650 else v['fontWeight'])} for k,v in tokens['typography'].items()},'size':tokens['size'],'motion':tokens['motion'],'elevation':tokens['elevation']}
(R/'tokens/native-tokens.ts').write_text('// Generated native data. Dimensions are dp; font values use scalable native units.\n// Adapt shadows to the actual native platform; no web CSS or fonts are imported.\nexport const nativeTokens = '+json.dumps(native,ensure_ascii=False,indent=2)+' as const;\nexport type ThemeMode = "light" | "dark";\nexport const resolveNativeTheme = (mode: ThemeMode) => ({...nativeTokens, colors: nativeTokens.colors[mode]});\n')
print('Generated CSS and TypeScript tokens for light/dark themes.')
