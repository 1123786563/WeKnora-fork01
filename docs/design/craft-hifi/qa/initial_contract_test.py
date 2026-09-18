from pathlib import Path
import unittest
ROOT=Path(__file__).resolve().parents[1]
class DeliveryContract(unittest.TestCase):
    def test_standalone_entry_exists(self):
        self.assertTrue((ROOT/'index.html').is_file(), '尚未实现高保真入口')
    def test_tokens_preserve_existing_brand(self):
        p=ROOT/'design/tokens.css'
        self.assertTrue(p.is_file(), '尚未实现令牌映射')
        text=p.read_text()
        for color in ['#2e6de6','#07c05f','#f7f9fc','#172033']:
            self.assertIn(color,text)
    def test_interactions_have_distinct_version_and_base(self):
        p=ROOT/'prototype/app.js'
        self.assertTrue(p.is_file(), '尚未实现状态交互')
        text=p.read_text()
        for key in ['viewVersion','baseVersion','workspaceRevision','runCount']:
            self.assertIn(key,text)
if __name__=='__main__': unittest.main()
