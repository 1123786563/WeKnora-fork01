import unittest

from scripts.saas.gate import validate_gate


class GateTests(unittest.TestCase):
    def test_schema_evidence_is_not_runtime_pass(self):
        with self.assertRaises(ValueError):
            validate_gate({"family": "official_v3", "checks": {"OM-02": "schema"}})

    def test_http_like_pass_without_evidence_is_rejected(self):
        report = {"family": "official_v3", "checks": {f"OM-{n:02d}": "pass" for n in range(1, 10)}}
        with self.assertRaises(ValueError):
            validate_gate(report)

    def test_schema_only_evidence_cannot_bypass_runtime_gate(self):
        report = {"family": "official_v3", "checks": {f"OM-{n:02d}": "pass" for n in range(1, 10)},
                  "evidence": [{"check": f"OM-{n:02d}", "status": "pass", "evidence_type": "schema_only"}
                               for n in range(1, 10)],
                  "settlement_ack_strategy": "query"}
        with self.assertRaises(ValueError):
            validate_gate(report)

    def test_blocked_experiment_cannot_authorize_model(self):
        report = {"family": "official_v3", "checks": {f"OM-{n:02d}": "blocked-env" for n in range(1, 10)},
                  "evidence": [{"status": "blocked-env"}], "settlement_ack_strategy": "query"}
        with self.assertRaises(ValueError):
            validate_gate(report)

    def test_complete_runtime_report_passes(self):
        report = {"family": "official_v1_v2", "checks": {f"OM-{n:02d}": "pass" for n in range(1, 10)},
                  "evidence": [{"check": f"OM-{n:02d}", "status": "pass", "result": {}}
                               for n in range(1, 10)],
                  "settlement_ack_strategy": "query by stable external id"}
        validate_gate(report)


if __name__ == "__main__":
    unittest.main()
