"""Offline regression tests for docker/craft/web/build.py (stdlib only).

Covers the OCR findings fixed in this change set:
- the sanitization variant set must close over BOTH decoders (HTML entity
  decoding and CSS backslash escapes), so an entity-encoded CSS escape
  smuggle is refused exactly like its literal form;
- pure-CJK headings must still yield UNIQUE filter/table ids (the section
  index participates), or every table's filter binds to the first input;
- the inline-event-handler denylist is anchored inside a tag opening, so
  prose like "only = 3" is not refused while real onclick= still is.
"""

import importlib.util
import unittest
from pathlib import Path

BUILD_PY = Path(__file__).resolve().parent / "build.py"


def load_build_module():
    spec = importlib.util.spec_from_file_location("craft_web_build_tested", BUILD_PY)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class BuildSanitizerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build = load_build_module()

    def test_entity_encoded_css_escape_smuggle_is_refused(self):
        # <div style="&#92;75 rl&#40;&#92;2f&#92;2fevil&#46;example&#41;">
        # decodes (entities) to \75 rl(\2f\2fevil.example) which decodes
        # (CSS) to url(//evil.example). Raw bytes pass every literal scan;
        # the composed closure must refuse it.
        fragment = '<div style="&#92;75 rl&#40;&#92;2f&#92;2fevil&#46;example&#41;">x</div>'
        with self.assertRaisesRegex(self.build.BuildError, "external URL|css url"):
            self.build.render_html("实体CSS组合绕过", fragment)

    def test_literal_css_escape_smuggle_is_refused(self):
        fragment = '<div style="\\75 rl(\\2f\\2fevil.example)">x</div>'
        with self.assertRaisesRegex(self.build.BuildError, "external URL|css url"):
            self.build.render_html("CSS字面绕过", fragment)

    def test_benign_fragment_passes(self):
        out = self.build.render_html("正常片段", '<p>hello &amp; welcome</p>')
        self.assertIn("hello &amp; welcome", out)

    def test_prose_with_only_equals_is_not_an_event_handler(self):
        # "only = 3" previously matched \bon[a-z]+\s*= and was refused.
        out = self.build.render_html("正文", "<p>only = 3, once=1, online=enabled</p>")
        self.assertIn("only = 3", out)

    def test_real_inline_handler_is_refused(self):
        with self.assertRaisesRegex(self.build.BuildError, "inline event handler"):
            self.build.render_html("真实内联", '<img src="x.png" onerror="alert(1)">')

    def test_cjk_headings_yield_unique_filter_and_table_ids(self):
        table_a = {"columns": ["列一"], "rows": [["a"]]}
        table_b = {"columns": ["列二"], "rows": [["b"]]}
        rendered_a = self.build.render_table("表格", table_a, 0)
        rendered_b = self.build.render_table("表格", table_b, 1)
        id_a = self.build.re.search(r'id="(craft-filter-[^"]+)"', rendered_a).group(1)
        id_b = self.build.re.search(r'id="(craft-filter-[^"]+)"', rendered_b).group(1)
        self.assertNotEqual(id_a, id_b, "two CJK-titled tables must not share a filter id")
        table_id_a = self.build.re.search(r'id="(craft-table-[^"]+)"', rendered_a).group(1)
        table_id_b = self.build.re.search(r'id="(craft-table-[^"]+)"', rendered_b).group(1)
        self.assertNotEqual(table_id_a, table_id_b)
        self.assertIn('aria-controls="{}"'.format(table_id_a), rendered_a)
        self.assertIn('aria-labelledby="{}"'.format(id_a), rendered_a)


if __name__ == "__main__":
    unittest.main()
