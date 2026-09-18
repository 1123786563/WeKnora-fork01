from pathlib import Path
import json
R=Path(__file__).resolve().parent
html=(R/'prototype/shell.html').read_text(encoding="utf-8")
for key,path in [('TOKENS','design/tokens.css'),('STYLES','prototype/app.css'),('APP','prototype/app.js')]:
    html=html.replace('/*'+key+'*/',(R/path).read_text(encoding="utf-8"))
icons=json.loads((R/'prototype/icons.json').read_text(encoding="utf-8"))
html=html.replace('/*ICONS*/','const ICONS='+json.dumps(icons,ensure_ascii=False)+';')
html=html.replace('aria-label="关闭面板">关闭</button>','aria-label="关闭面板">'+icons['close']+'</button>')
(R/'index.html').write_text(html, encoding="utf-8")
print('Built',R/'index.html',len(html.encode()),'bytes')
