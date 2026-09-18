from pathlib import Path
import json,re,unittest
R=Path(__file__).resolve().parents[1]
def lum(c):
 vals=[int(c[i:i+2],16)/255 for i in (1,3,5)]
 v=[x/12.92 if x<=.04045 else ((x+.055)/1.055)**2.4 for x in vals]
 return sum(a*b for a,b in zip(v,[.2126,.7152,.0722]))
def ratio(a,b):
 x,y=sorted([lum(a),lum(b)]);return (y+.05)/(x+.05)
d=json.loads((R/'design/resolved-tokens.json').read_text())
pairs=[('正文/白卡','color.text.primary','color.bg.surface'),('正文/画布','color.text.primary','color.bg.page'),('次文字/白卡','color.text.secondary','color.bg.surface'),('第三级/画布','color.text.tertiary','color.bg.page'),('主按钮白文字','color.text.inverse','color.action.primary'),('深色卡白文字','color.text.inverse','color.bg.brand'),('成功状态','color.status.successFg','color.status.successBg'),('警告状态','color.status.warningFg','color.status.warningBg'),('错误状态','color.status.dangerFg','color.status.dangerBg'),('信息状态','color.status.infoFg','color.status.infoBg')]
results=[{'pair':n,'foreground':d[f],'background':d[b],'ratio':round(ratio(d[f],d[b]),3),'threshold':4.5,'passed':ratio(d[f],d[b])>=4.5} for n,f,b in pairs]
(R/'tests/contrast-results.json').write_text(json.dumps(results,ensure_ascii=False,indent=2)+'\n')
class DesignTests(unittest.TestCase):
 def test_contrast_pairs(self):
  self.assertTrue(all(x['passed'] for x in results),str([x for x in results if not x['passed']]))
 def test_token_generation(self):
  self.assertEqual(len(d),106)
  css=(R/'design/tokens.css').read_text();scss=(R/'design/_tokens.taro.scss').read_text()
  self.assertIn('--space-16:16px;',css);self.assertIn('$space-16: 32px;',scss)
  self.assertIn('$component-hairline: 1Px;',scss)
  self.assertNotRegex((R/'design/resolved-tokens.json').read_text(),r'\{color\.')
 def test_page_task_mapping(self):
  ids={t['id'] for t in json.loads((R/'tasks/task-index.json').read_text())}
  mapping=json.loads((R/'prototype/developer-mapping.json').read_text());self.assertEqual(len(mapping),20)
  for p,m in mapping.items():
   self.assertTrue(m['tasks'],p)
   self.assertTrue(all(x in ids for x in m['tasks']),p)
 def test_all_tasks_reach_release(self):
  tasks={t['id']:t for t in json.loads((R/'tasks/task-index.json').read_text())};anc=set()
  def walk(n):
   if n in anc:return
   anc.add(n)
   for d in tasks[n]['dependencies']:walk(d)
  walk('MP04-T03-06');self.assertEqual(set(tasks),anc)
 def test_standalone_no_remote_assets(self):
  for p in list((R/'prototype/pages').glob('*.html'))+[R/'prototype/index.html',R/'design/components.html']:
   text=p.read_text()
   self.assertFalse(re.search(r'<(?:script|img|iframe)[^>]+src=["\']https?://',text,re.I),str(p))
   self.assertFalse(re.search(r'@import\s+url\(',text,re.I),str(p))
if __name__=='__main__':unittest.main()
