from __future__ import annotations

import json
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from native_baseline import documents_terminal_with_graph, load_frozen_cases, run_lifecycle, verify_persisted_graph_config
from native_env import NativeEnvConfig


class NativeBaselineTest(unittest.TestCase):
    def test_documents_require_completed_status_and_no_pending_subtasks(self) -> None:
        rows = [{"id": "d-1", "parse_status": "completed", "pending_subtasks_count": 1}]
        self.assertFalse(documents_terminal_with_graph(rows, ["d-1"]))
        rows[0]["pending_subtasks_count"] = 0
        self.assertTrue(documents_terminal_with_graph(rows, ["d-1"]))

    def test_persisted_config_requires_enabled_node_extract(self) -> None:
        with self.assertRaisesRegex(RuntimeError, "nodeExtract"):
            verify_persisted_graph_config({"data": {"nodeExtract": {"enabled": False}}})
        verify_persisted_graph_config({"data": {"nodeExtract": {"enabled": True}}})

    def test_lifecycle_retains_sanitized_failure_and_always_tears_down(self) -> None:
        with TemporaryDirectory() as directory:
            config = NativeEnvConfig.from_mapping({"run_id": "unit-life", "artifact_dir": directory})
            output = Path(directory) / "result.json"
            calls: list[str] = []
            def fail_provision(_: NativeEnvConfig) -> dict:
                raise RuntimeError("provision failed: token=secret")
            exit_code = run_lifecycle(config, output, up_fn=lambda _: calls.append("up") or 0, provision_fn=fail_provision, teardown_fn=lambda _: calls.append("teardown") or 0)
            result = json.loads(output.read_text())
            self.assertEqual(exit_code, 1)
            self.assertEqual(calls, ["up", "teardown"])
            self.assertEqual(result["status"], "failed")
            self.assertEqual(result["failure_stage"], "provision")
            self.assertNotIn("secret", result["error"])

    def test_lifecycle_marks_zero_graph_references_as_negative_evidence(self) -> None:
        with TemporaryDirectory() as directory:
            config = NativeEnvConfig.from_mapping({"run_id": "unit-negative", "artifact_dir": directory})
            output = Path(directory) / "result.json"
            def complete(_: NativeEnvConfig) -> dict:
                return {"status": "completed", "evidence_ids": [], "references": [], "raw_sse": "event: message\\ndata: {}\\n\\n"}
            exit_code = run_lifecycle(config, output, up_fn=lambda _: 0, provision_fn=complete, teardown_fn=lambda _: 0)
            result = json.loads(output.read_text())
            self.assertEqual(exit_code, 1)
            self.assertEqual(result["status"], "negative")
            self.assertEqual(result["evidence_ids"], [])

    def test_lifecycle_fails_when_teardown_returns_nonzero(self) -> None:
        with TemporaryDirectory() as directory:
            config = NativeEnvConfig.from_mapping({"run_id": "unit-teardown", "artifact_dir": directory})
            output = Path(directory) / "result.json"
            def complete(_: NativeEnvConfig) -> dict:
                return {"status": "completed", "evidence_ids": ["e-d1"], "references": []}
            exit_code = run_lifecycle(config, output, up_fn=lambda _: 0, provision_fn=complete, teardown_fn=lambda _: 7)
            result = json.loads(output.read_text())
            self.assertEqual(exit_code, 1)
            self.assertEqual(result["status"], "failed")
            self.assertEqual(result["failure_stage"], "teardown")
            self.assertEqual(result["teardown_failure"], "teardown exited 7")

    def test_frozen_dataset_keeps_deleted_and_revoked_sources_as_runtime_inputs(self) -> None:
        with TemporaryDirectory() as directory:
            dataset = Path(directory) / "cases.jsonl"
            dataset.write_text(
                '{"case_id":"deleted","document_revision":"v","documents":[{"evidence_id":"public","text":"公开"}],"forbidden_sources":[{"evidence_id":"secret","state":"deleted","text":"CANARY"}]}\n'
                '{"case_id":"revoked","document_revision":"v","documents":[{"evidence_id":"public","text":"公开"}],"forbidden_sources":[{"evidence_id":"secret2","state":"revoked","text":"CANARY2"}]}\n',
                encoding="utf-8",
            )
            rows = load_frozen_cases(dataset)
        self.assertEqual([row["forbidden_sources"][0]["state"] for row in rows], ["deleted", "revoked"])


if __name__ == "__main__":
    unittest.main()
