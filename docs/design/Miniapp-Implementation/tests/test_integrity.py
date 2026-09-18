import json,unittest
from pathlib import Path
R=Path(__file__).resolve().parents[1]
class DeliveryTests(unittest.TestCase):
 def test_pages(self):
  p=R/'prototype/page-manifest.json';self.assertTrue(p.exists())
  pages=json.loads(p.read_text());self.assertEqual(len(pages),20)
  for p in pages:self.assertTrue((R/'prototype/pages'/p['file']).exists())
 def test_tokens(self):
  for n in ['tokens.json','tokens.css','_tokens.taro.scss','tokens.ts','components.html']:self.assertTrue((R/'design'/n).exists(),n)
 def test_tasks(self):
  p=R/'tasks/task-index.json';self.assertTrue(p.exists())
  ts=json.loads(p.read_text());self.assertEqual(len(ts),59)
  graph={t['id']:t['dependencies'] for t in ts};self.assertEqual(len(graph),59)
  seen=set();active=set()
  def visit(n):
   self.assertIn(n,graph);self.assertNotIn(n,active)
   if n in seen:return
   active.add(n)
   for d in graph[n]:visit(d)
   active.remove(n);seen.add(n)
  for t in ts:
   visit(t['id']);self.assertTrue(t['locks']);self.assertGreaterEqual(len(t['acceptance']),3)
   self.assertTrue((R/'tasks'/(t['id']+'.md')).exists())
 def test_doc_set(self):
  for n in ['01-前端详细设计.md','02-后端与共享契约详细设计.md','03-视觉与组件规范.md','04-页面详细设计.md','05-任务拆分与并行计划.md']:
   self.assertTrue((R/'docs'/n).exists(),n)
if __name__=='__main__':unittest.main()
