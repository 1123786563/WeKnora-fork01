import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

from scripts.saas.release_gate import (
    PROVIDER_CASES,
    capability_prefixes,
    main,
    parse_required_ids,
    require_pass,
    unfinished_scope,
)

REPO_ROOT = Path(__file__).resolve().parents[2]
SPEC = REPO_ROOT / "docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md"
DESIGN = REPO_ROOT / "docs/superpowers/specs/2026-09-10-saas-billing-connectors-design.md"


def passing(level="unit"):
    return {"status": "pass", "level": level, "evidence": ["ledger commit", "directed test command"]}


class RequirePassTests(unittest.TestCase):
    def test_mock_cannot_prove_provider_acceptance(self):
        with self.assertRaises(ValueError):
            require_pass({'WX-05':{'status':'pass','level':'mock','evidence':['x']}},{'WX-05'})

    def test_missing_case_is_failure(self):
        with self.assertRaises(ValueError): require_pass({}, {'AC-01'})

    def test_skip_status_is_rejected(self):
        with self.assertRaises(ValueError):
            require_pass({"OM-01": {"status": "skip", "level": "provider", "evidence": ["x"]}}, {"OM-01"})

    def test_blocked_env_status_is_rejected(self):
        with self.assertRaises(ValueError):
            require_pass(
                {"OM-02": {"status": "blocked-env", "level": "provider", "evidence": ["no service"]}},
                {"OM-02"},
            )

    def test_pass_without_evidence_is_rejected(self):
        with self.assertRaises(ValueError):
            require_pass({"COM-01": {"status": "pass", "level": "unit"}}, {"COM-01"})

    def test_pass_with_empty_evidence_list_is_rejected(self):
        with self.assertRaises(ValueError):
            require_pass({"COM-01": {"status": "pass", "level": "unit", "evidence": []}}, {"COM-01"})

    def test_provider_cases_require_provider_level(self):
        for level in ("unit", "integration", "browser", "mock"):
            with self.assertRaises(ValueError, msg=level):
                require_pass({key: passing(level) for key in PROVIDER_CASES}, set(PROVIDER_CASES))

    def test_provider_case_error_names_real_provider_requirement(self):
        with self.assertRaises(ValueError) as ctx:
            require_pass({"NO-04": passing("integration")}, {"NO-04"})
        self.assertIn("real provider evidence required", str(ctx.exception))

    def test_non_provider_cases_accept_unit_and_integration_levels(self):
        records = {key: passing("unit") for key in ("COM-01", "BUD-01", "USE-01")}
        records.update({key: passing("integration") for key in ("CON-01", "SYNC-01", "OPS-01")})
        require_pass(records, set(records))

    def test_provider_case_passes_with_provider_level(self):
        require_pass({"WX-05": passing("provider")}, {"WX-05"})


class ParseRequiredIdsTests(unittest.TestCase):
    def test_real_spec_yields_sixty_six_interface_ids_plus_twenty_acs(self):
        ids = parse_required_ids(SPEC, DESIGN)
        interface = {i for i in ids if not i.startswith("AC-")}
        self.assertEqual(66, len(interface))
        expected = {"ALI": 5, "BUD": 8, "COM": 7, "CON": 8, "FS": 4, "NO": 4,
                    "OM": 10, "OPS": 4, "SYNC": 5, "USE": 6, "WX": 5}
        for group, count in expected.items():
            actual = len([i for i in interface if i.startswith(group + "-")])
            self.assertEqual(count, actual, group)
        self.assertEqual({f"AC-{number:02d}" for number in range(1, 21)}, ids - interface)

    def test_parses_ids_from_markdown_row_format(self):
        with tempfile.TemporaryDirectory() as tmp:
            spec = Path(tmp) / "spec.md"
            design = Path(tmp) / "design.md"
            spec.write_text(
                "| ID | 检查 | 通过标准 |\n| --- | --- | --- |\n"
                "| WX-05 | 真实受控联验 | x |\n| G0 环境与契约 | OM-01、OM-02 | y |\n",
                encoding="utf-8",
            )
            design.write_text(
                "| 编号 | 场景 |\n| --- | --- |\n| AC-01 | 场景 |\n| AC-20 | 场景 |\n| B01 | 旧表 |\n",
                encoding="utf-8",
            )
            self.assertEqual({"WX-05", "AC-01", "AC-20"}, parse_required_ids(spec, design))


class ExclusionTests(unittest.TestCase):
    def test_without_capability_removes_ids_but_lists_them_unfinished(self):
        ids = {"OM-01", "OM-02", "COM-01"}
        excluded = capability_prefixes({"om"}, ids)
        self.assertEqual({"OM-01", "OM-02"}, excluded)
        records = {"COM-01": passing("unit")}
        require_pass(records, ids - excluded)
        unfinished = unfinished_scope(records, ids, excluded)
        self.assertEqual(["OM-01", "OM-02"], [row["id"] for row in unfinished])
        self.assertTrue(all(row["reason"] == "excluded" for row in unfinished))

    def test_non_passing_required_rows_are_listed_with_status(self):
        ids = {"OM-01", "WX-05", "COM-01"}
        records = {"OM-01": {"status": "blocked-env", "level": "provider", "evidence": []},
                   "WX-05": {"status": "pending", "level": "provider", "evidence": []},
                   "COM-01": passing("unit")}
        unfinished = unfinished_scope(records, ids, set())
        self.assertEqual(["OM-01", "WX-05"], [row["id"] for row in unfinished])
        self.assertEqual("blocked-env", unfinished[0]["status"])


class MainTests(unittest.TestCase):
    def _write_inputs(self, tmp, records):
        spec = Path(tmp) / "spec.md"
        design = Path(tmp) / "design.md"
        report = Path(tmp) / "report.json"
        spec.write_text("| ID | 检查 | 通过标准 |\n| --- | --- | --- |\n| OM-01 | x | y |\n| COM-01 | x | y |\n", encoding="utf-8")
        design.write_text("| 编号 | 场景 |\n| --- | --- |\n| AC-01 | s |\n", encoding="utf-8")
        report.write_text(json.dumps({"records": records}), encoding="utf-8")
        return spec, design, report

    def test_main_exits_1_on_blocked_env(self):
        with tempfile.TemporaryDirectory() as tmp:
            spec, design, report = self._write_inputs(
                tmp, {"OM-01": {"status": "blocked-env", "level": "provider", "evidence": []},
                      "COM-01": passing("unit"), "AC-01": passing("unit")})
            out = io.StringIO()
            with redirect_stdout(out):
                code = main(["--report", str(report), "--spec", str(spec), "--design", str(design)])
            self.assertEqual(1, code)
            self.assertIn("unfinished: OM-01", out.getvalue())

    def test_main_exits_0_with_exclusion_but_lists_excluded_scope(self):
        with tempfile.TemporaryDirectory() as tmp:
            spec, design, report = self._write_inputs(tmp, {"COM-01": passing("unit"), "AC-01": passing("unit")})
            out = io.StringIO()
            with redirect_stdout(out):
                code = main(["--report", str(report), "--spec", str(spec), "--design", str(design),
                             "--without", "om"])
            self.assertEqual(0, code)
            self.assertIn("OM-01", out.getvalue())
            self.assertIn("excluded", out.getvalue())


if __name__ == "__main__":
    unittest.main()
