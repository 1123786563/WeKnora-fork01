import importlib.util
import os
import json
from pathlib import Path
import sys
import subprocess
import tempfile
from unittest import mock
import unittest

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
spec = importlib.util.spec_from_file_location("concurrent_evidence", HERE / "concurrent_consumption_86.py")
cc = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cc)
os.environ.setdefault("LAGO_API_KEY", "unit-test-only")
cspec = importlib.util.spec_from_file_location("consume_evidence", HERE / "consume_86.py")
consume = importlib.util.module_from_spec(cspec)
cspec.loader.exec_module(consume)
rspec = importlib.util.spec_from_file_location("reconcile_evidence", HERE / "reconcile.py")
reconcile = importlib.util.module_from_spec(rspec)
rspec.loader.exec_module(reconcile)

class EvidenceHelpersTest(unittest.TestCase):
    def test_consume_help_does_not_discover_credentials_or_call_git(self):
        with tempfile.TemporaryDirectory() as root:
            bin_dir = Path(root) / "bin"
            bin_dir.mkdir()
            marker = Path(root) / "git-called"
            shim = bin_dir / "git"
            shim.write_text("#!/bin/sh\nprintf called > %s\nexit 1\n" % marker)
            shim.chmod(0o755)
            env = dict(os.environ)
            env.pop("LAGO_API_KEY", None)
            env["PATH"] = str(bin_dir)
            result = subprocess.run([sys.executable, str(HERE / "consume_86.py"), "--help"],
                                     cwd=root, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(marker.exists())

    def test_consume_api_key_discovery_failures_are_bounded(self):
        failures = (subprocess.CalledProcessError(1, ["git"]), OSError("git missing"))
        for failure in failures:
            with self.subTest(failure=type(failure).__name__), \
                 mock.patch.dict(os.environ, {}, clear=True), \
                 mock.patch.object(consume.subprocess, "run", side_effect=failure):
                with self.assertRaisesRegex(SystemExit, "api key not found") as caught:
                    consume.api_key()
                self.assertNotIn("Traceback", str(caught.exception))
        with tempfile.TemporaryDirectory() as root:
            for content in (None, "WEKNORA_COMMERCIAL_PLATFORM_API_KEY=\n"):
                env_path = Path(root) / ".env"
                if content is None:
                    env_path.unlink(missing_ok=True)
                else:
                    env_path.write_text(content)
                with self.subTest(content=content), \
                     mock.patch.dict(os.environ, {}, clear=True), \
                     mock.patch.object(consume.subprocess, "run", return_value=mock.Mock(stdout=root + "\n")):
                    with self.assertRaisesRegex(SystemExit, "api key not found"):
                        consume.api_key()

    def test_consume_request_uses_fallback_key_only_in_authorization_header(self):
        with tempfile.TemporaryDirectory() as root:
            (Path(root) / ".env").write_text("WEKNORA_COMMERCIAL_PLATFORM_API_KEY=fallback-secret\n")
            response = mock.MagicMock()
            response.__enter__.return_value.status = 200
            response.__enter__.return_value.read.return_value = b"{}"
            with mock.patch.dict(os.environ, {}, clear=True), \
                 mock.patch.object(consume.subprocess, "run", return_value=mock.Mock(stdout=root + "\n")), \
                 mock.patch.object(consume.OPENER, "open", return_value=response) as open_call:
                self.assertEqual(consume.request("GET", "/probe"), (200, {}))
            request = open_call.call_args.args[0]
            self.assertEqual(request.get_header("Authorization"), "Bearer fallback-secret")
            self.assertNotIn("fallback-secret", str(request.full_url))

    def test_consume_missing_key_does_not_open_network(self):
        with tempfile.TemporaryDirectory() as root, \
             mock.patch.dict(os.environ, {}, clear=True), \
             mock.patch.object(consume.subprocess, "run", return_value=mock.Mock(stdout=root + "\n")), \
             mock.patch.object(consume.OPENER, "open") as open_call:
            with self.assertRaisesRegex(SystemExit, "api key not found"):
                consume.request("GET", "/probe")
            open_call.assert_not_called()

    def test_consume_price_to_cents_accepts_exact_positive_cents(self):
        self.assertEqual(consume.price_to_cents("1.00"), 100)
        self.assertEqual(consume.price_to_cents("1.5"), 150)

    def test_consume_price_to_cents_rejects_invalid_amounts(self):
        for value in ("0.995", "1.00000000000000000000000000001", "0", "-1.00", "not-money", "NaN", "Infinity"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                consume.price_to_cents(value)

    def test_consume_runner_failure_persists_redacted_evidence(self):
        for mode in ("timeout", "wrong_draw", "missing_seed", "duplicate_seed", "event_failure"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as root:
                out = Path(root) / "run"
                before = {"weknora-2026-09": 100, "weknora-topup-c": 500,
                          "weknora-topup-d": 500}
                after = dict(before)
                if mode == "timeout":
                    after["weknora-2026-09"] = 0
                elif mode == "wrong_draw":
                    after["weknora-topup-d"] -= 200
                elif mode == "missing_seed":
                    before.pop("weknora-2026-09")
                    after = dict(before)
                elif mode == "duplicate_seed":
                    before["other-2026-09"] = 100
                    after = dict(before)
                def fake_request(method, path, payload=None):
                    if path.endswith("/events") and mode == "event_failure":
                        return 500, {"detail": "RAW_RESPONSE_SENTINEL", "key": os.environ["LAGO_API_KEY"]}
                    if path.endswith("/events"):
                        return 201, {}
                    if path.endswith("/billable_metrics"):
                        return 201, {"billable_metric": {"lago_id": "metric"}}
                    if path.endswith("/plans"):
                        return 201, {"plan": {"lago_id": "plan"}}
                    return 201, {"subscription": {"status": "active"}}
                with mock.patch.object(consume, "request", side_effect=fake_request), \
                     mock.patch.object(consume, "balance_snapshot", side_effect=[before, after]), \
                     mock.patch.object(consume, "wait_until", return_value=(mode != "timeout", 0.1)), \
                     mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                    result = consume.main()
                self.assertNotEqual(result, 0)
                facts = json.loads((out / "consume-cny.json").read_text())
                self.assertEqual(facts["verdict"], "FAIL")
                self.assertTrue(facts["failed_stage"])
                self.assertLessEqual(len(facts["error"]["message"]), 300)
                self.assertIn("before", facts["observations"])
                self.assertNotIn(os.environ["LAGO_API_KEY"], json.dumps(facts))
                self.assertNotIn("RAW_RESPONSE_SENTINEL", json.dumps(facts))
                if mode == "timeout":
                    self.assertEqual(facts["failed_stage"], "settlement")

    def test_consume_runner_failure_artifact_writer_error_is_bounded(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            capture = __import__("io").StringIO()
            with mock.patch.object(consume, "request", side_effect=RuntimeError("RAW_RESPONSE_SENTINEL " + os.environ["LAGO_API_KEY"])), \
                 mock.patch.object(consume.tempfile, "NamedTemporaryFile", side_effect=OSError("RAW_RESPONSE_SENTINEL " + os.environ["LAGO_API_KEY"])), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                 mock.patch.object(sys, "stderr", capture):
                result = consume.main()
            self.assertNotEqual(result, 0)
            self.assertLessEqual(len(capture.getvalue()), 400)
            self.assertNotIn(os.environ["LAGO_API_KEY"], capture.getvalue())
            self.assertNotIn("RAW_RESPONSE_SENTINEL", capture.getvalue())

    def test_consume_cli_propagates_nonzero_status_without_network(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            env = dict(os.environ, LAGO_API_KEY="cli-test-key", LAGO_BASE="ftp://127.0.0.1:48889")
            proc = subprocess.run([sys.executable, str(HERE / "consume_86.py"), "--output-dir", str(out)],
                                  env=env, capture_output=True, text=True, timeout=10)
            self.assertNotEqual(proc.returncode, 0)
            facts = json.loads((out / "consume-cny.json").read_text())
            self.assertEqual(facts["verdict"], "FAIL")

    def test_consume_failed_atomic_pass_publish_leaves_fail_artifact_and_no_temp(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            before = {"x-2026-09": 100, "topup-c": 500, "topup-d": 500}
            after = {"x-2026-09": 0, "topup-c": 400, "topup-d": 500}
            def fake_request(method, path, payload=None):
                if path.endswith("billable_metrics"):
                    return 201, {"billable_metric": {"lago_id": "metric"}}
                if path.endswith("/plans"):
                    return 201, {"plan": {"lago_id": "plan"}}
                if path.endswith("/subscriptions"):
                    return 201, {"subscription": {"status": "active"}}
                return 201, {}
            real_replace = os.replace
            calls = 0
            def fail_pass_publish(src, dst):
                nonlocal calls
                calls += 1
                if calls == 1:
                    raise OSError("publish failure")
                return real_replace(src, dst)
            with mock.patch.object(consume, "request", side_effect=fake_request), \
                 mock.patch.object(consume, "balance_snapshot", side_effect=[before, after]), \
                 mock.patch.object(consume, "wait_until", return_value=(True, 0.1)), \
                 mock.patch.object(consume.os, "replace", side_effect=fail_pass_publish), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = consume.main()
            self.assertNotEqual(result, 0)
            facts = json.loads((out / "consume-cny.json").read_text())
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertEqual(list(out.glob("*.tmp")), [])

    def test_consume_keyboard_interrupt_persists_fail_and_returns_130(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            with mock.patch.object(consume, "request", side_effect=KeyboardInterrupt()), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = consume.main()
            self.assertEqual(result, 130)
            facts = json.loads((out / "consume-cny.json").read_text())
            self.assertEqual(facts["verdict"], "FAIL")
    def test_decode_error_response_accepts_non_json_body(self):
        self.assertIsNone(cc.decode_error_body(b"<html>bad gateway</html>"))
        self.assertIsNone(cc.decode_error_body(b"{broken"))
        err = __import__("urllib.error", fromlist=["HTTPError"]).HTTPError(
            "http://127.0.0.1", 502, "bad", {}, __import__("io").BytesIO(b"<html>"))
        with mock.patch.object(cc.OPENER, "open", side_effect=err):
            self.assertEqual(cc.call("GET", "/probe"), (502, None))

    def test_wallet_view_uses_one_active_row_snapshot_for_balance_and_rank(self):
        rows = [{"name": "active", "status": "active", "balance_cents": 8,
                 "expiration_at": "e", "created_at": "c", "lago_id": "id"},
                {"name": "dead", "status": "terminated", "balance_cents": 4,
                 "expiration_at": "x", "created_at": "y", "lago_id": "z"}]
        balances, ranks = cc.active_wallet_view(rows)
        self.assertEqual(balances, {"active": 8})
        self.assertEqual(ranks, {"active": ("e", "c", "id")})

    def test_concurrent_output_preflight_errors_are_bounded_and_help_is_unchanged(self):
        for error in (FileExistsError("already exists"), FileNotFoundError("parent missing")):
            with self.subTest(error=type(error).__name__), tempfile.TemporaryDirectory() as root:
                stderr = __import__("io").StringIO()
                with mock.patch.object(sys, "argv", ["runner", "--output-dir", str(Path(root) / "run")]), \
                     mock.patch.object(cc, "prepare_output_dir", side_effect=error), \
                     mock.patch.object(sys, "stderr", stderr):
                    self.assertEqual(cc.main(), 2)
                self.assertEqual(stderr.getvalue().count("FAIL: output-dir preflight:"), 1)
                self.assertNotIn("Traceback", stderr.getvalue())
        with mock.patch.object(sys, "argv", ["runner", "--help"]), self.assertRaises(SystemExit) as caught:
            cc.main()
        self.assertEqual(caught.exception.code, 0)

    def test_concurrent_postcommit_transcript_failures_preserve_pass_contract(self):
        class SelectiveFailure:
            def __init__(self, fragment=None):
                self.fragment, self.parts = fragment, []
            def write(self, value):
                if self.fragment and self.fragment in value:
                    raise OSError("transcript write failed")
                self.parts.append(value)
            def flush(self):
                return None

        for failure in ("FACTS_JSON", "CONCURRENT CONSUMPTION PASS", "context-exit"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as root:
                out = Path(root) / "run"
                stdout, stderr = SelectiveFailure(None if failure == "context-exit" else failure), __import__("io").StringIO()
                def committed_probe(output, state):
                    cc.write_facts_atomic(output, {"verdict": "PASS"})
                    state["committed"] = True
                    print("FACTS_JSON {}")
                    print("CONCURRENT CONSUMPTION PASS")
                    return 0
                from contextlib import contextmanager
                @contextmanager
                def context_exit_failure(capture):
                    try:
                        yield
                        if failure == "context-exit":
                            capture.flush()
                            capture.close()
                            raise OSError("flush failed")
                        capture.flush()
                    finally:
                        if not capture.closed:
                            capture.close()
                with mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                     mock.patch.object(cc, "run_probe", side_effect=committed_probe), \
                     mock.patch.object(cc, "capture_stdout", side_effect=context_exit_failure), \
                     mock.patch.object(sys, "stdout", stdout), mock.patch.object(sys, "stderr", stderr):
                    result = cc.main()
                self.assertEqual(result, 0)
                self.assertEqual(json.loads((out / "facts.json").read_text()), {"verdict": "PASS"})
                self.assertNotIn("FAIL", "".join(stdout.parts))
                self.assertEqual(stderr.getvalue(), "WARNING: evidence transcript incomplete (OSError)\n")

    def test_concurrent_facts_writer_is_atomic_and_cleans_catchable_failures(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root)
            for failure_point in ("dump", "flush", "close", "replace"):
                with self.subTest(failure_point=failure_point):
                    destination = out / "facts.json"
                    destination.unlink(missing_ok=True)
                    if failure_point == "dump":
                        patcher = mock.patch.object(cc.json, "dump", side_effect=OSError("write failed"))
                    elif failure_point == "flush":
                        patcher = mock.patch.object(cc, "_flush_facts_file", side_effect=OSError("flush failed"))
                    elif failure_point == "close":
                        patcher = mock.patch.object(cc, "_close_facts_file", side_effect=OSError("close failed"))
                    else:
                        patcher = mock.patch.object(cc.os, "replace", side_effect=OSError("rename failed"))
                    with patcher, self.assertRaises(OSError):
                        cc.write_facts_atomic(out, {"verdict": "PASS"})
                    self.assertFalse(destination.exists())
                    self.assertEqual(list(out.glob(".facts-*.tmp")), [])

    def test_concurrent_interrupted_facts_write_never_exposes_partial_canonical_file(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            out.mkdir()
            code = ("import sys; sys.path.insert(0, %r); import concurrent_consumption_86 as m; "
                    "m.json.dump=lambda facts, fh, **kw: (fh.write('{\\\"verdict\\\":'), __import__('os')._exit(17)); "
                    "m.write_facts_atomic(%r, {'verdict':'PASS'})") % (str(HERE), str(out))
            proc = subprocess.run([sys.executable, "-c", code], timeout=10)
            self.assertEqual(proc.returncode, 17)
            canonical = out / "facts.json"
            self.assertFalse(canonical.exists())

    def test_concurrent_main_never_emits_pass_before_successful_facts_publish(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            stdout, stderr = __import__("io").StringIO(), __import__("io").StringIO()
            real_replace = os.replace
            facts_replace_attempts = []
            def fail_first_replace(src, dst):
                if Path(dst).name == "facts.json":
                    facts_replace_attempts.append((src, dst))
                if Path(dst).name == "facts.json" and len(facts_replace_attempts) == 1:
                    raise OSError("publish failed")
                return real_replace(src, dst)
            responses = [(201, {"billable_metric": {"lago_id": "metric"}}),
                         (201, {"plan": {"lago_id": "plan"}}),
                         (201, {"subscription": {"lago_id": "sub"}})]
            def fake_call(method, path, payload=None):
                if path.endswith("/events"):
                    return 201, {"event": {"transaction_id": payload["event"]["transaction_id"]}}
                return responses.pop(0)
            before, after = {"wallet": 500}, {"wallet": 0}
            rows = [{"name": "wallet", "status": "active", "balance_cents": 0,
                     "expiration_at": "2027-01-01T00:00:00Z",
                     "created_at": "2026-01-01T00:00:00Z", "lago_id": "wallet-id"}]
            with mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                 mock.patch.object(cc, "call", side_effect=fake_call), \
                 mock.patch.object(cc, "balances", side_effect=[before, after]), \
                 mock.patch.object(cc, "wallet_list", return_value=rows), \
                 mock.patch.object(cc, "wait_until", return_value=(True, 0.01)), \
                 mock.patch.object(cc, "observe_replay_balance", return_value=(True, after, 300.0)), \
                 mock.patch.object(cc, "replay_response_ok", return_value=True), \
                 mock.patch.object(cc.os, "replace", side_effect=fail_first_replace), \
                 mock.patch.object(sys, "stdout", stdout), mock.patch.object(sys, "stderr", stderr):
                result = cc.main()
            self.assertNotEqual(result, 0)
            self.assertNotIn("CONCURRENT CONSUMPTION PASS", stdout.getvalue())
            transcript = (out / "concurrent-output.txt").read_text()
            self.assertNotIn("CONCURRENT CONSUMPTION PASS", transcript)
            facts = json.loads((out / "facts.json").read_text())
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertEqual(len(facts["checks"]), 6)
            self.assertTrue(all(check["passed"] for check in facts["checks"]))
            self.assertEqual(facts["failed_stage"], "duplicate_replay_observation")
            self.assertEqual(len(facts_replace_attempts), 2)
            self.assertEqual(list(out.glob(".facts-*.tmp")), [])

    def test_duplicate_replay_requires_pinned_duplicate_identity(self):
        for status, body in ((200, {"status": "ok"}),
                             (201, {"event": {"transaction_id": "different-id"}}),
                             (422, {"errors": [{"code": "unprocessable_entity"}]}),
                             (422, None), (400, {"message": "bad"}), (500, {"error": "oops"})):
            self.assertFalse(cc.replay_response_ok(status, body))
            summary = cc.replay_response_summary(status, body)
            self.assertNotIn("oops", json.dumps(summary))
            self.assertNotIn("different-id", json.dumps(summary))

    def test_reconcile_api_key_requires_lago_api_key(self):
        with mock.patch.dict(os.environ, {"LAGO_API_KEY": "secret"}, clear=True):
            self.assertEqual(reconcile.api_key(), "secret")

    def test_reconcile_api_key_rejects_empty_lago_api_key(self):
        with mock.patch.dict(os.environ, {"LAGO_API_KEY": ""}, clear=True):
            with self.assertRaisesRegex(RuntimeError, "LAGO_API_KEY"):
                reconcile.api_key()

    def test_runner_assertion_failure_uses_common_finalizer(self):
        self._run_failed_probe("assertion", checks_expected=6)

    def test_runner_precheck_system_exit_uses_common_finalizer(self):
        self._run_failed_probe("precheck", checks_expected=0)

    def test_runner_unexpected_zero_system_exit_persists_redacted_fail_facts(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            original_stdout = sys.stdout
            with mock.patch.dict(os.environ, {"LAGO_API_KEY": "sentinel-secret"}), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                 mock.patch.object(cc, "run_probe", side_effect=SystemExit(0)):
                result = cc.main()
            self.assertNotEqual(result, 0)
            self.assertIs(sys.stdout, original_stdout)
            facts_text = (out / "facts.json").read_text()
            facts = json.loads(facts_text)
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertNotIn("sentinel-secret", facts_text)
            self.assertNotIn("response body", facts_text)


    def test_runner_capture_open_failure_is_best_effort(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            original = sys.stdout
            real_open = Path.open
            def fail_capture(path, *args, **kwargs):
                if path.name == "concurrent-output.txt":
                    raise OSError("secret open failure")
                return real_open(path, *args, **kwargs)
            with mock.patch.object(Path, "open", autospec=True, side_effect=fail_capture), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = cc.main()
            self.assertNotEqual(result, 0)
            self.assertIs(sys.stdout, original)
            facts = json.loads((out / "facts.json").read_text())
            self.assertEqual(facts["checks"], [])
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertFalse((out / "concurrent-output.txt").exists())

    def test_runner_replay_call_order_and_rank_change_fail_closed(self):
        self._run_failed_probe("rank-change", checks_expected=6)

    def test_runner_observation_failure_preserves_duplicate_response(self):
        self._run_failed_probe("observer-failure", checks_expected=0)

    def test_runner_replay_request_failure_has_accurate_stage(self):
        self._run_failed_probe("replay-request-failure", checks_expected=0)

    def _run_failed_probe(self, mode, checks_expected):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            responses = [(201, {"billable_metric": {"lago_id": "metric"}}),
                         (201, {"plan": {"lago_id": "plan"}}),
                         (201, {"subscription": {"lago_id": "sub"}})]
            calls = []
            facts_writes = []
            original_replace = os.replace
            def track_facts_publish(src, dst):
                if Path(dst).name == "facts.json":
                    facts_writes.append(Path(dst))
                return original_replace(src, dst)
            event_posts = 0
            def fake_call(method, path, payload=None):
                nonlocal event_posts
                if path.endswith("/events"):
                    event_posts += 1
                    calls.append("event_replay" if event_posts == 6 else "event_post")
                    if mode == "replay-request-failure" and event_posts == 6:
                        raise RuntimeError("unit-test-only replay failure")
                    if event_posts == 6:
                        return 422, {"message": "sensitive response body"}
                if not path.endswith("/events"):
                    calls.append(path)
                if path.endswith("/events"):
                    return 201, {}
                return responses.pop(0)
            rows = ([{"name": "wallet-a", "status": "active", "balance_cents": 0,
                      "expiration_at": "e", "created_at": "c", "lago_id": "id-a"},
                     {"name": "wallet-b", "status": "terminated", "balance_cents": 100,
                      "expiration_at": "f", "created_at": "d", "lago_id": "id-b"}]
                    if mode == "rank-change" else
                    [{"name": "wallet", "status": "active", "balance_cents": 20,
                      "expiration_at": "e", "created_at": "c", "lago_id": "id"}])
            wallet_sequence = []
            def wallet_read():
                calls.append("wallet_read")
                wallet_sequence.append("wallet_read")
                return rows
            def balances():
                return {"wallet-a": 500, "wallet-b": 100} if mode == "rank-change" else {"wallet": 20}
            observer = mock.Mock(return_value=(True, {"wallet": 20}, 300.0))
            if mode == "observer-failure":
                observer.side_effect = RuntimeError("unit-test-only observer failure")
            original_stdout = sys.stdout
            def stop_before_checks(*args, **kwargs):
                raise SystemExit("unit-test-only precheck")
            with mock.patch.object(cc, "call", side_effect=fake_call), \
                 mock.patch.object(cc, "balances", side_effect=balances), \
                 mock.patch.object(cc, "wallet_list", side_effect=wallet_read), \
                 mock.patch.object(cc, "wait_until", side_effect=lambda predicate, timeout: (predicate(), 0.1)), \
                 mock.patch.object(cc, "observe_replay_balance", observer), \
                 mock.patch.object(cc, "draw_order_ok", wraps=cc.draw_order_ok), \
                 mock.patch.object(cc, "replay_response_ok", return_value=False), \
                 mock.patch.object(cc.os, "replace", side_effect=track_facts_publish), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                if mode == "precheck":
                    with mock.patch.object(cc, "run_probe", side_effect=SystemExit("unit-test-only precheck")):
                        result = cc.main()
                else:
                    result = cc.main()
            self.assertNotEqual(result, 0)
            self.assertIs(sys.stdout, original_stdout)
            facts = json.loads((out / "facts.json").read_text())
            self.assertEqual(len(facts_writes), 1)
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertTrue(facts["failed_stage"])
            self.assertLessEqual(len(facts["error"]["message"]), 300)
            self.assertNotIn("unit-test-only", facts["error"]["message"])
            self.assertEqual(len(facts["checks"]), checks_expected)
            if mode == "assertion":
                self.assertNotIn("checks", facts.get("observations", {}))
                self.assertEqual(len(facts["checks"]), 6)
            if mode != "precheck":
                self.assertIn("before", facts["observations"])
                self.assertTrue((out / "concurrent-output.txt").read_text())
                if mode == "rank-change":
                    self.assertLess(calls.index("wallet_read"), calls.index("event_replay"))
                    rank_check = next(c for c in facts["checks"] if c["name"].startswith("draw order follows expiry rank"))
                    self.assertFalse(rank_check["passed"])
                    self.assertEqual(rank_check["evidence"], "wallet identity set changed: before=['wallet-a', 'wallet-b'] after=['wallet-a']")
                if mode == "observer-failure":
                    self.assertEqual(facts["failed_stage"], "duplicate_replay_observation")
                    self.assertEqual(facts["observations"]["replay_status"], 422)
                    self.assertEqual(facts["observations"]["replay_response"], {
                        "status": 422, "body_type": "dict", "duplicate_identity": "unproven"})
                    self.assertNotIn("sensitive response body", json.dumps(facts))
                    self.assertNotIn(os.environ["LAGO_API_KEY"], json.dumps(facts))
                if mode == "replay-request-failure":
                    self.assertEqual(facts["failed_stage"], "duplicate_replay_request")

    def test_capture_stdout_restores_and_closes_when_body_raises(self):
        import io
        original = sys.stdout
        capture = io.StringIO()
        with self.assertRaisesRegex(RuntimeError, "boom"):
            with cc.capture_stdout(capture):
                print("captured")
                raise RuntimeError("boom")
        self.assertIs(sys.stdout, original)
        self.assertTrue(capture.closed)

    def test_settled_snapshot_is_captured_before_replay_and_reused_as_baseline(self):
        rows = [{"name": "a", "status": "active", "balance_cents": 0,
                 "expiration_at": "2027", "created_at": "c", "lago_id": "a"},
                {"name": "b", "status": "active", "balance_cents": 100,
                 "expiration_at": "2028", "created_at": "d", "lago_id": "b"}]
        calls = []
        create = [(201, {"billable_metric": {"lago_id": "metric"}}),
                  (201, {"plan": {"lago_id": "plan"}}),
                  (201, {"subscription": {"lago_id": "sub"}})]
        def fake_call(method, path, payload=None):
            calls.append(path)
            if path.endswith("billable_metrics"):
                return create.pop(0)
            if path.endswith("/plans") or path.endswith("/subscriptions"):
                return create.pop(0)
            return 200, None
        before = {"a": 500, "b": 100}
        after = {"a": 0, "b": 100}
        observer_args = []
        balance_reads = [before, before, after]
        def fake_observer(expected):
            observer_args.append(expected)
            return True, dict(expected), 300.0
        with tempfile.TemporaryDirectory() as root, \
             mock.patch.object(cc, "call", side_effect=fake_call), \
             mock.patch.object(cc, "balances", side_effect=balance_reads), \
             mock.patch.object(cc, "wallet_list", return_value=rows) as read_rows, \
             mock.patch.object(cc, "wait_until", side_effect=lambda predicate, timeout: (predicate(), 0.1)), \
             mock.patch.object(cc, "observe_replay_balance", side_effect=fake_observer), \
             mock.patch.object(sys, "argv", ["runner", "--output-dir", str(Path(root) / "run")]):
            result = cc.main()
            facts = json.loads((Path(root) / "run" / "facts.json").read_text())
            self.assertEqual(result, 1)
        self.assertEqual(read_rows.call_count, 1)
        self.assertEqual(observer_args, [after])
        self.assertEqual(facts["observations"]["after"], after)
        self.assertEqual(facts["observations"]["replay_after"], after)
        self.assertEqual(facts["verdict"], "FAIL")
        self.assertFalse(facts["checks"][0]["passed"])

    def test_concurrent_early_setup_failure_preserves_run_codes(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            calls = []
            def fake_call(method, path, payload=None):
                calls.append((method, path, payload))
                if path.endswith("/billable_metrics"):
                    return 201, {"billable_metric": {"lago_id": "provider-metric-id"}}
                if path.endswith("/plans"):
                    return 500, {"detail": "PROVIDER_BODY_SENTINEL", "key": "PROVIDER_SECRET_SENTINEL"}
                self.fail("unexpected provider request: %s" % path)
            prior_stdout = sys.stdout
            with mock.patch.object(cc, "call", side_effect=fake_call), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = cc.main()
            self.assertNotEqual(result, 0)
            self.assertIs(sys.stdout, prior_stdout)
            self.assertEqual([path.rsplit("/", 1)[-1] for _, path, _ in calls],
                             ["billable_metrics", "plans"])
            facts = json.loads((out / "facts.json").read_text())
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertEqual(facts["failed_stage"], "create_plan")
            observations = facts["observations"]
            candidates = observations["generated_candidates"]
            self.assertEqual(set(candidates), {"tag", "metric_code", "plan_code", "subscription_code"})
            tag = candidates["tag"]
            self.assertEqual(candidates["metric_code"], "weknora-86-metric-" + tag)
            self.assertEqual(candidates["plan_code"], "weknora-86-plan-" + tag)
            self.assertEqual(candidates["subscription_code"], "weknora-86-sub-" + tag)
            self.assertEqual(observations["created_resources"], {"metric_code": candidates["metric_code"]})
            serialized = json.dumps(facts)
            self.assertNotIn("provider-metric-id", serialized)
            self.assertNotIn("PROVIDER_BODY_SENTINEL", serialized)
            self.assertNotIn("PROVIDER_SECRET_SENTINEL", serialized)

    def test_concurrent_subscription_setup_failure_preserves_confirmed_codes(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            calls = []
            responses = [
                (201, {"billable_metric": {"lago_id": "provider-metric-id"}}),
                (201, {"plan": {"lago_id": "provider-plan-id"}}),
                (503, {"detail": "PROVIDER_BODY_SENTINEL"}),
            ]
            def fake_call(method, path, payload=None):
                calls.append(path)
                return responses.pop(0)
            with mock.patch.object(cc, "call", side_effect=fake_call), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = cc.main()
            self.assertNotEqual(result, 0)
            self.assertEqual([path.rsplit("/", 1)[-1] for path in calls],
                             ["billable_metrics", "plans", "subscriptions"])
            facts = json.loads((out / "facts.json").read_text())
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertEqual(facts["failed_stage"], "create_subscription")
            observations = facts["observations"]
            candidates = observations["generated_candidates"]
            self.assertEqual(observations["created_resources"], {
                "metric_code": candidates["metric_code"],
                "plan_code": candidates["plan_code"],
            })
            self.assertNotIn("subscription_code", observations["created_resources"])
            serialized = json.dumps(facts)
            for marker in ("provider-metric-id", "provider-plan-id", "PROVIDER_BODY_SENTINEL"):
                self.assertNotIn(marker, serialized)

    def test_runner_event_post_failure_writes_redacted_failed_facts(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            responses = [(201, {"billable_metric": {"lago_id": "metric"}}),
                         (201, {"plan": {"lago_id": "plan"}}),
                         (201, {"subscription": {"lago_id": "sub"}})]
            def fake_call(method, path, payload=None):
                if path.endswith("/events"):
                    return 500, {"detail": "unit-test-only"}
                return responses.pop(0)
            rows = [{"name": "wallet", "status": "active", "balance_cents": 20,
                     "expiration_at": "e", "created_at": "c", "lago_id": "id"}]
            prior_stdout = sys.stdout
            with mock.patch.object(cc, "call", side_effect=fake_call), \
                 mock.patch.object(cc, "balances", return_value={"wallet": 20}), \
                 mock.patch.object(cc, "wallet_list", return_value=rows), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = cc.main()
            self.assertNotEqual(result, 0)
            self.assertIs(sys.stdout, prior_stdout)
            facts = json.loads((out / "facts.json").read_text())
            self.assertEqual(facts["verdict"], "FAIL")
            self.assertEqual(facts["failed_stage"], "post_events")
            candidates = facts["observations"]["generated_candidates"]
            self.assertEqual(facts["observations"]["created_resources"], {
                "metric_code": candidates["metric_code"],
                "plan_code": candidates["plan_code"],
                "subscription_code": candidates["subscription_code"],
            })
            self.assertIn("before", facts["observations"])
            self.assertNotIn("unit-test-only", json.dumps(facts))
            self.assertTrue((out / "concurrent-output.txt").exists())
            transcript = (out / "concurrent-output.txt").read_text()
            self.assertIn("event posts:", transcript)

    def test_all_three_readers_follow_lago_page_metadata(self):
        pages = {
            1: {"wallets": [{"lago_id": "w1", "name": "w1", "status": "active", "balance_cents": 1}],
                "meta": {"current_page": 1, "next_page": 2, "total_pages": 2, "total_count": 2}},
            2: {"wallets": [{"lago_id": "w2", "name": "w2", "status": "terminated", "balance_cents": 0}],
                "meta": {"current_page": 2, "next_page": None, "total_pages": 2, "total_count": 2}},
        }
        for reader in (cc.wallet_list, consume.wallets, reconcile.lago_wallets):
            requested = []
            rows = reader(fetch_page=lambda page: (requested.append(page) or pages[page]))
            self.assertEqual(requested, [1, 2])
            if isinstance(rows, dict):
                rows = list(rows.values())
            self.assertEqual([w["lago_id"] for w in rows], ["w1", "w2"])

    def test_wallet_list_reports_http_status(self):
        with mock.patch.object(cc, "call", return_value=(503, None)):
            with self.assertRaisesRegex(RuntimeError, "HTTP 503"):
                cc.wallet_list()

    def test_wallet_list_rejects_non_2xx_without_response_content(self):
        body = {"error": "provider-marker SECRET-MARKER"}
        with mock.patch.object(cc, "call", return_value=(503, body)):
            with self.assertRaises(RuntimeError) as caught:
                cc.wallet_list()
        self.assertNotIn("provider-marker", str(caught.exception))
        self.assertNotIn("SECRET-MARKER", str(caught.exception))

    def test_wallet_list_accepts_2xx_page(self):
        wallet = {"lago_id": "w1", "name": "w1", "status": "active", "balance_cents": 1}
        body = {"wallets": [wallet],
                "meta": {"current_page": 1, "next_page": None, "total_pages": 1, "total_count": 1}}
        with mock.patch.object(cc, "call", return_value=(200, body)):
            self.assertEqual(cc.wallet_list(), [wallet])

    def test_all_readers_reject_truncated_or_malformed_pagination(self):
        bad_pages = [
            {"wallets": [{"lago_id": "w1"}], "meta": {"current_page": 1, "next_page": None, "total_pages": 2, "total_count": 2}},
            {"wallets": [], "meta": {"current_page": 9, "next_page": None, "total_pages": 1, "total_count": 0}},
            {"wallets": [], "meta": {"current_page": 1, "next_page": None, "total_pages": 1, "total_count": 1}},
            {"wallets": [], "meta": {"current_page": 1, "total_pages": 1, "total_count": 0}},
        ]
        for reader in (cc.wallet_list, consume.wallets, reconcile.lago_wallets):
            for body in bad_pages:
                with self.assertRaises((ValueError, RuntimeError)):
                    reader(fetch_page=lambda page, body=body: body)

    def test_checked_in_lago_fixture_uses_page_not_cursor_metadata(self):
        fixture = __import__("json").loads((HERE / "lago-baseline-tenant10000.json").read_text())
        rows = cc.read_wallet_pages(lambda page: fixture)
        self.assertEqual(len(rows), fixture["meta"]["total_count"])
        self.assertNotIn("next_cursor", fixture["meta"])

    def test_reader_rejects_duplicate_wallet_identity(self):
        body = {"wallets": [{"lago_id": "w1", "name": "same"},
                            {"lago_id": "w2", "name": "same"}],
                "meta": {"current_page": 1, "next_page": None,
                         "total_pages": 1, "total_count": 2}}
        with self.assertRaises(ValueError):
            cc.read_wallet_pages(lambda page: body)

    def test_consumption_snapshots_exclude_terminated_wallets(self):
        rows = [{"name": "live", "status": "active", "balance_cents": 5},
                {"name": "dead", "status": "terminated", "balance_cents": 100}]
        with mock.patch.object(cc, "wallet_list", return_value=rows):
            self.assertEqual(cc.balances(), {"live": 5})
        with mock.patch.object(consume, "wallets", return_value={w["name"]: w for w in rows}):
            self.assertEqual(consume.balance_snapshot(), {"live": 5})

    def test_cascade_rank_uses_expiry_grant_and_stable_identity(self):
        before = {"z": 0, "b": 100, "a": 100, "c": 100}
        after = {"z": 0, "b": 0, "a": 0, "c": 50}
        rank = {"z": ("2026-01", "2025-01", "z"), "b": ("2027-01", "2025-02", "b"), "a": ("2027-01", "2025-03", "a"), "c": ("2028-01", "2025-01", "c")}
        self.assertTrue(cc.draw_order_ok(before, after, 250, rank)[0])
        # Equal-expiry wallets are ordered by grant timestamp, then stable ID.
        tie_rank = {"first": ("2027", "2025-01", "b-id"),
                    "second": ("2027", "2025-01", "c-id")}
        self.assertTrue(cc.draw_order_ok({"first": 10, "second": 10},
                                         {"first": 0, "second": 10},
                                         10, tie_rank)[0])

    def test_cascade_detail_reports_last_remaining_balance(self):
        before = {"first": 100, "second": 100, "last": 500}
        after = {"first": 0, "second": 0, "last": 400}
        rank = {name: ("2026-%02d" % i, "2025-01", name)
                for i, name in enumerate(before, 1)}
        ok, detail = cc.draw_order_ok(before, after, 300, rank)
        self.assertTrue(ok)
        self.assertIn("last_remaining=400", detail)
        self.assertNotIn("spill=400", detail)

    def test_rejects_skipped_wallet_missing_rank_changed_set_and_overdraw(self):
        rank = {n: ("2027", "2025", n) for n in "abc"}
        self.assertFalse(cc.draw_order_ok(dict(a=100,b=100,c=100), dict(a=50,b=100,c=50), 100, rank)[0])
        self.assertFalse(cc.draw_order_ok(dict(a=100), dict(a=0), 100, {})[0])
        self.assertFalse(cc.draw_order_ok(dict(a=100), dict(a=0,b=5), 100, {"a":("1","1","a"),"b":("2","1","b")})[0])
        self.assertFalse(cc.draw_order_ok(dict(a=100), dict(a=-1), 100, {"a":("1","1","a")})[0])
        self.assertFalse(cc.draw_order_ok(dict(a=100), dict(a=0), 101, {"a":("1","1","a")})[0])
        self.assertFalse(cc.draw_order_ok(dict(a=100), dict(a=0), 100, {"a":("","1","a")})[0])

    def test_terminated_residual_wallets_fail_closed(self):
        with self.assertRaises(ValueError):
            reconcile.assert_no_terminated_residuals([{"status":"terminated", "balance_cents":1}])
        reconcile.assert_no_terminated_residuals([{"status":"terminated", "balance_cents":0}])

    def test_output_directory_must_be_new(self):
        with tempfile.TemporaryDirectory() as root:
            output = Path(root) / "run-1"
            self.assertEqual(cc.prepare_output_dir(output), output)
            with self.assertRaises(FileExistsError):
                cc.prepare_output_dir(output)

    def test_concurrent_setup_requires_success_status_and_identifier(self):
        self.assertEqual(cc.require_created(201, {"plan": {"lago_id": "p1"}}, "plan", "test"), "p1")
        with self.assertRaises(RuntimeError):
            cc.require_created(422, {"plan": {"lago_id": "p1"}}, "plan", "test")
        with self.assertRaises(RuntimeError):
            cc.require_created(201, {"plan": {}}, "plan", "test")
        self.assertFalse(cc.replay_response_ok(200, None))
        self.assertFalse(cc.replay_response_ok(422, None))
        self.assertFalse(cc.replay_response_ok(500, None))

    def test_replay_observer_detects_late_per_wallet_change_with_same_total(self):
        expected = {"wallet-a": 50, "wallet-b": 50}
        changed = {"wallet-a": 40, "wallet-b": 60}
        with mock.patch.object(cc, "balances", side_effect=[expected, changed]), \
             mock.patch.object(cc.time, "monotonic", side_effect=[0.0, 0.0, 0.5]), \
             mock.patch.object(cc.time, "sleep"):
            stable, observed, elapsed = cc.observe_replay_balance(expected, observe_seconds=3, interval=1)
        self.assertFalse(stable)
        self.assertEqual(observed, changed)
        self.assertEqual(sum(observed.values()), sum(expected.values()))
        self.assertEqual(elapsed, 0.5)

    def test_replay_observer_samples_at_full_bound(self):
        expected = {"wallet-a": 20}
        with mock.patch.object(cc, "balances", side_effect=[expected, expected]) as read_balances, \
             mock.patch.object(cc.time, "monotonic", side_effect=[0.0, 0.0, 3.0]), \
             mock.patch.object(cc.time, "sleep"):
            stable, observed, elapsed = cc.observe_replay_balance(expected, observe_seconds=3, interval=2)
        self.assertTrue(stable)
        self.assertEqual(observed, expected)
        self.assertEqual(elapsed, 3.0)
        self.assertEqual(read_balances.call_count, 2)

    def test_final_replay_snapshot_rejects_same_total_wallet_redistribution(self):
        expected = {"wallet-a": 50, "wallet-b": 50}
        final = {"wallet-a": 40, "wallet-b": 60}
        self.assertEqual(sum(expected.values()), sum(final.values()))
        self.assertFalse(cc.replay_snapshot_matches(expected, final))
        self.assertTrue(cc.replay_snapshot_matches(expected, expected.copy()))

if __name__ == '__main__': unittest.main()

class ReconcileTask2Test(unittest.TestCase):
    period = "2026-09"
    month_end = "2026-10-01T00:00:00Z"

    def monthly_wallet(self, **overrides):
        row = {"name": "weknora-tenant-10000-2026-09", "status": "active",
               "balance_cents": 100, "expiration_at": self.month_end,
               "created_at": "2026-09-01T00:00:00Z",
               "metadata": {"weknora_tenant": "weknora-tenant-10000",
                            "weknora_period": self.period}}
        row.update(overrides)
        return row

    def monthly_page(self, **overrides):
        row = {"source": "monthly", "period": self.period,
               "balance_micro": 1_000_000, "expires_at": self.month_end,
               "granted_at": "2026-09-01T00:00:00Z"}
        row.update(overrides)
        return row

    def topup_wallet(self, **overrides):
        row = {"name": "topup-a", "status": "active", "balance_cents": 500,
               "expiration_at": "2027-09-01T00:00:00Z",
               "created_at": "2026-01-01T00:00:00Z",
               "metadata": {"weknora_tenant": "weknora-tenant-10000"}}
        row.update(overrides)
        return row

    def topup_page(self, **overrides):
        row = {"source": "topup", "balance_micro": 5_000_000,
               "expires_at": "2027-09-01T00:00:00Z",
               "granted_at": "2026-01-01T00:00:00Z"}
        row.update(overrides)
        return row

    def test_reconcile_monthly_aggregates_both_markers_by_period(self):
        first = self.monthly_wallet()
        second = self.monthly_wallet(name="purchase", balance_cents=25,
                                     created_at="2026-09-03T00:00:00Z",
                                     metadata={"weknora_tenant": "weknora-tenant-10000",
                                               "weknora_purchase_period": self.period})
        page = self.monthly_page(balance_micro=1_250_000)
        self.assertEqual(reconcile.reconcile_batches([page], [first, second]), (True, []))

    def test_reconcile_rejects_conflicting_markers_and_unanchored_wallet(self):
        conflict = self.monthly_wallet(metadata={"weknora_tenant": "weknora-tenant-10000",
                                                 "weknora_period": self.period,
                                                 "weknora_purchase_period": "2026-08"})
        ok, errors = reconcile.reconcile_batches([], [conflict])
        self.assertFalse(ok)
        self.assertTrue(errors)
        unanchored = self.topup_wallet(metadata={})
        ok, errors = reconcile.reconcile_batches([], [unanchored])
        self.assertFalse(ok)
        self.assertTrue(errors)

    def test_reconcile_resolves_both_deterministic_monthly_names(self):
        for name in ("weknora-tenant-10000-2026-09",
                     "weknora-tenant-10000-purchase-2026-09"):
            wallet = self.monthly_wallet(name=name, metadata={"weknora_tenant": "weknora-tenant-10000"})
            self.assertEqual(reconcile.reconcile_batches([self.monthly_page()], [wallet]), (True, []))

    def test_reconcile_matches_equal_expiry_topups_by_complete_unique_face(self):
        a = self.topup_wallet(name="a")
        b = self.topup_wallet(name="b", balance_cents=250, created_at="2026-02-01T00:00:00Z")
        pages = [self.topup_page(), self.topup_page(balance_micro=2_500_000, granted_at="2026-02-01T00:00:00Z")]
        self.assertEqual(reconcile.reconcile_batches(pages, [a, b]), (True, []))

    def test_reconcile_rejects_duplicate_topup_faces_as_ambiguous(self):
        a, b = self.topup_wallet(name="a"), self.topup_wallet(name="b")
        ok, errors = reconcile.reconcile_batches([self.topup_page(), self.topup_page()], [a, b])
        self.assertFalse(ok)
        self.assertIn("ambiguous", " ".join(errors).lower())

    def test_reconcile_rejects_extra_missing_or_changed_topup_face(self):
        wallet = self.topup_wallet()
        self.assertFalse(reconcile.reconcile_batches([], [wallet])[0])
        self.assertFalse(reconcile.reconcile_batches([self.topup_page(), self.topup_page(granted_at="other")], [wallet])[0])
        self.assertFalse(reconcile.reconcile_batches([self.topup_page(balance_micro=5_000_001)], [wallet])[0])

    def test_reconcile_requires_active_zero_topup_to_match(self):
        wallet = self.topup_wallet(balance_cents=0)
        self.assertFalse(reconcile.reconcile_batches([], [wallet])[0])
        self.assertEqual(reconcile.reconcile_batches([self.topup_page(balance_micro=0)], [wallet]), (True, []))

    def test_reconcile_allows_only_page_only_zero_orphans(self):
        monthly = self.monthly_page(period="2026-08", balance_micro=0,
                                    expires_at="2026-09-01T00:00:00Z")
        topup = self.topup_page(balance_micro=0)
        self.assertEqual(reconcile.reconcile_batches([monthly], []), (True, []))
        self.assertEqual(reconcile.reconcile_batches([topup], []), (True, []))
        for orphan in (monthly, topup):
            for balance in (1, "not-an-integer"):
                ok, errors = reconcile.reconcile_batches([dict(orphan, balance_micro=balance)], [])
                self.assertFalse(ok)
                self.assertTrue(errors)

    def test_reconcile_labels_negative_monthly_orphan_as_nonzero(self):
        orphan = self.monthly_page(period="2026-08", balance_micro=-1,
                                   expires_at="2026-09-01T00:00:00Z")
        ok, errors = reconcile.reconcile_batches([orphan], [])
        self.assertFalse(ok)
        self.assertIn("nonzero page-only monthly orphan", errors)
        self.assertNotIn("positive page-only monthly orphan", errors)

    def test_integer_accepts_ascii_signed_decimal_strings_and_rejects_other_shapes(self):
        for value, expected in (("4000000", 4000000), (12, 12), ("-1", -1), (-1, -1)):
            self.assertEqual(reconcile._integer(value, "value"), expected)
        for value in (True, 1.0, "+1", "-", " 1", "١", "1x"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                reconcile._integer(value, "value")

    def test_reconcile_accepts_documented_string_micro_values(self):
        wallet = self.monthly_wallet(balance_cents=100)
        page = self.monthly_page(balance_micro="1000000")
        with tempfile.TemporaryDirectory() as root:
            account = Path(root) / "account.json"
            out = Path(root) / "run"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [page], "balance_micro": "1000000",
                "available_micro": "1000000", "held_micro": "0",
                "refund_locked_micro": "0"}}}}))
            with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                 mock.patch.object(reconcile, "lago_wallets", return_value=[wallet]), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = reconcile.main()
            self.assertEqual(result, 0)
            self.assertIn("RECONCILE PASS", (out / "reconcile-output.txt").read_text())

    def test_reconcile_terminated_wallet_missing_balance_fails_canonically(self):
        account_data = {"data": {"benefits": {"credits": {
            "batches": [], "balance_micro": "0", "available_micro": "0",
            "held_micro": "0", "refund_locked_micro": "0"}}}}
        with tempfile.TemporaryDirectory() as root:
            account, out = Path(root) / "account.json", Path(root) / "run"
            account.write_text(json.dumps(account_data))
            with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                 mock.patch.object(reconcile, "lago_wallets", return_value=[{"status": "terminated"}]), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                self.assertNotEqual(reconcile.main(), 0)
            report = (out / "reconcile-output.txt").read_text()
            self.assertIn("RECONCILE FAIL", report)
            self.assertIn("stage=validation", report)
            self.assertIn("ValueError", report)
            self.assertNotIn("[PASS] terminated wallets", report)
        reconcile.assert_no_terminated_residuals([{"status": "terminated", "balance_cents": 0}])
        with self.assertRaises(ValueError):
            reconcile.assert_no_terminated_residuals([{"status": "terminated", "balance_cents": 1}])

    def test_reconcile_report_preserves_first_eight_complete_batch_errors(self):
        labels = ["fixed error label %02d" % i for i in range(1, 11)]
        with tempfile.TemporaryDirectory() as root:
            account, out = Path(root) / "account.json", Path(root) / "run"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [], "balance_micro": "0", "available_micro": "0",
                "held_micro": "0", "refund_locked_micro": "0"}}}}))
            with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                 mock.patch.object(reconcile, "lago_wallets", return_value=[]), \
                 mock.patch.object(reconcile, "reconcile_batches", return_value=(False, labels)), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                self.assertEqual(reconcile.main(), 1)
            report = (out / "reconcile-output.txt").read_text()
        for label in labels[:8]:
            self.assertIn(label, report)
        for label in labels[8:]:
            self.assertNotIn(label, report)

    def test_negative_account_and_available_micro_values_are_reconciled(self):
        with tempfile.TemporaryDirectory() as root:
            account = Path(root) / "account.json"
            out = Path(root) / "run"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [], "balance_micro": "0", "available_micro": "-1",
                "held_micro": "1", "refund_locked_micro": "0"}}}}))
            with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                 mock.patch.object(reconcile, "lago_wallets", return_value=[]), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                result = reconcile.main()
            artifact = (out / "reconcile-output.txt").read_text()
            self.assertEqual(result, 0)
            self.assertIn("-1 == 0 - 1 - 0", artifact)
            self.assertIn("RECONCILE PASS", artifact)

    def test_reconcile_checks_monthly_registered_expiry_and_terminated_residuals(self):
        wallet = self.monthly_wallet(expiration_at="2026-09-30T23:00:00Z")
        self.assertFalse(reconcile.reconcile_batches([self.monthly_page()], [wallet])[0])
        self.assertFalse(reconcile.reconcile_batches(
            [self.monthly_page(expires_at="2026-09-30T23:00:00Z")],
            [self.monthly_wallet()])[0])
        with self.assertRaises(ValueError):
            reconcile.assert_no_terminated_residuals([{"status": "terminated", "balance_cents": 1}])
        reconcile.assert_no_terminated_residuals([{"status": "terminated", "balance_cents": 0}])

    def test_anchored_customer_prefixed_nonperiod_wallet_name_is_topup(self):
        wallet = self.topup_wallet(name="weknora-tenant-10000-topup-123")
        self.assertEqual(reconcile.reconcile_batches([self.topup_page()], [wallet]), (True, []))

    def test_malformed_explicit_monthly_period_marker_fails(self):
        wallet = self.monthly_wallet(metadata={"weknora_tenant": "weknora-tenant-10000",
                                              "weknora_period": "2026-13"})
        ok, errors = reconcile.reconcile_batches([], [wallet])
        self.assertFalse(ok)
        self.assertTrue(errors)

    def test_reconcile_rejects_invalid_page_source_period_families(self):
        monthly = self.monthly_wallet()
        topup = self.topup_wallet()
        cases = [
            ([self.topup_page(source="topup", period=self.period)], [monthly]),
            ([self.monthly_page(period="")], [topup]),
            ([self.topup_page(source="unexpected", period="")], [topup]),
            ([self.monthly_page(source="unexpected")], [monthly]),
        ]
        for page, wallets in cases:
            with self.subTest(page=page[0]):
                ok, errors = reconcile.reconcile_batches(page, wallets)
                self.assertFalse(ok)
                self.assertTrue(any("family" in error.lower() or "source" in error.lower()
                                    for error in errors))

    def _run_reconcile_with_output_fault(self, fault):
        import tempfile
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            account = Path(root) / "account.json"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [], "balance_micro": 0, "available_micro": 0,
                "held_micro": 0, "refund_locked_micro": 0}}}}))
            capture = __import__("io").StringIO()
            stdout = __import__("io").StringIO()
            real_factory = tempfile.NamedTemporaryFile
            real_replace = reconcile.os.replace
            calls = {"factory": 0, "replace": 0}

            class WriterProxy:
                def __init__(self, stream):
                    self.stream = stream
                def __enter__(self):
                    return self
                def __exit__(self, exc_type, exc, tb):
                    result = self.stream.__exit__(exc_type, exc, tb)
                    if fault == "close" and calls["factory"] == 1:
                        raise OSError("LAGO_SENTINEL close body")
                    return result
                def write(self, value):
                    result = self.stream.write(value)
                    if fault == "write" and calls["factory"] == 1:
                        raise OSError("LAGO_SENTINEL write body")
                    return result
                def __getattr__(self, name):
                    return getattr(self.stream, name)

            def factory(*args, **kwargs):
                calls["factory"] += 1
                return WriterProxy(real_factory(*args, **kwargs))

            def replace(source, destination):
                calls["replace"] += 1
                if fault == "publish" and calls["replace"] == 1:
                    raise OSError("LAGO_SENTINEL publish body")
                return real_replace(source, destination)

            with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account),
                                              "LAGO_API_KEY": "LAGO_SENTINEL"}), \
                 mock.patch.object(reconcile, "lago_wallets", return_value=[]), \
                 mock.patch.object(tempfile, "NamedTemporaryFile", side_effect=factory), \
                 mock.patch.object(reconcile.os, "replace", side_effect=replace), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                 mock.patch.object(sys, "stdout", stdout), \
                 mock.patch.object(sys, "stderr", capture):
                result = reconcile.main()
            artifact = (out / "reconcile-output.txt").read_text() if (out / "reconcile-output.txt").exists() else ""
            self.assertNotEqual(result, 0)
            self.assertIn("RECONCILE FAIL", artifact)
            self.assertNotIn("RECONCILE PASS", artifact)
            self.assertNotIn("LAGO_SENTINEL", artifact + capture.getvalue())
            self.assertNotIn("write body", artifact + capture.getvalue())
            self.assertNotIn("close body", artifact + capture.getvalue())
            self.assertNotIn("publish body", artifact + capture.getvalue())
            self.assertNotIn("RECONCILE PASS", stdout.getvalue())
            self.assertEqual(list(out.glob("*.tmp")), [])
            self.assertFalse(any(path.name.startswith(".reconcile-") for path in out.iterdir()))

    def test_reconcile_artifact_write_failure_after_pass_staging_is_redacted(self):
        self._run_reconcile_with_output_fault("write")

    def test_reconcile_artifact_close_failure_after_pass_staging_is_redacted(self):
        self._run_reconcile_with_output_fault("close")

    def test_reconcile_artifact_publication_failure_after_pass_staging_is_redacted(self):
        self._run_reconcile_with_output_fault("publish")

    def _run_reconcile_with_stdout_fault(self, verdict, error):
        import tempfile
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            account = Path(root) / "account.json"
            page = [] if verdict == "PASS" else [self.topup_page()]
            total = 0 if verdict == "PASS" else 5_000_000
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": page, "balance_micro": total, "available_micro": total,
                "held_micro": 0, "refund_locked_micro": 0}}}}))
            artifact_path = out / "reconcile-output.txt"

            class FailingTextStream:
                def __init__(self):
                    self.artifact_at_failure = None
                def write(self, value):
                    if artifact_path.exists():
                        self.artifact_at_failure = artifact_path.read_bytes()
                    raise error("stdout sentinel")

            stdout = FailingTextStream()
            with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                 mock.patch.object(reconcile, "lago_wallets", return_value=[]), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                 mock.patch.object(sys, "stdout", stdout):
                result = reconcile.main()
                isolated_stdout = sys.stdout
            if isolated_stdout is not stdout:
                isolated_stdout.close()
            final = artifact_path.read_bytes()
            self.assertIsNotNone(stdout.artifact_at_failure)
            self.assertEqual(final, stdout.artifact_at_failure)
            self.assertIn("RECONCILE " + verdict, final.decode())
            self.assertEqual(result, 0 if verdict == "PASS" else 1)

    def test_published_pass_survives_stdout_value_error(self):
        self._run_reconcile_with_stdout_fault("PASS", ValueError)

    def test_published_fail_survives_stdout_os_error(self):
        self._run_reconcile_with_stdout_fault("FAIL", OSError)

    def test_published_pass_survives_stdout_flush_os_error(self):
        import tempfile
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            account = Path(root) / "account.json"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [], "balance_micro": 0, "available_micro": 0,
                "held_micro": 0, "refund_locked_micro": 0}}}}))
            artifact_path = out / "reconcile-output.txt"

            class FlushFailingTextStream:
                def __init__(self):
                    self.artifact_at_failure = None
                def write(self, value):
                    return len(value)
                def flush(self):
                    self.artifact_at_failure = artifact_path.read_bytes()
                    raise OSError("stdout flush sentinel")

            stdout = FlushFailingTextStream()
            with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                 mock.patch.object(reconcile, "lago_wallets", return_value=[]), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                 mock.patch.object(sys, "stdout", stdout):
                result = reconcile.main()
                isolated_stdout = sys.stdout
            if isolated_stdout is not stdout:
                isolated_stdout.close()
            final = artifact_path.read_bytes()
            self.assertEqual(final, stdout.artifact_at_failure)
            self.assertIn("RECONCILE PASS", final.decode())
            self.assertEqual(result, 0)

    def test_published_pass_survives_stdout_attribute_and_isolation_errors(self):
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            account = Path(root) / "account.json"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [], "balance_micro": 0, "available_micro": 0,
                "held_micro": 0, "refund_locked_micro": 0}}}}))
            class NoWriteOrFlush:
                pass

            class WriteFailure:
                def write(self, value):
                    raise OSError("stdout write failed")

            for index, (stream, isolate) in enumerate(((NoWriteOrFlush(), False), (WriteFailure(), True))):
                out = Path(root) / ("run-%d" % index)
                artifact_path = out / "reconcile-output.txt"
                with self.subTest(isolation_failure=isolate), \
                     mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                     mock.patch.object(reconcile, "lago_wallets", return_value=[]), \
                     mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                     mock.patch.object(sys, "stdout", stream):
                    if isolate:
                        with mock.patch.object(reconcile, "_isolate_failed_stdout",
                                               side_effect=RuntimeError("isolation failed")):
                            result = reconcile.main()
                    else:
                        result = reconcile.main()
                    isolated_stdout = sys.stdout
                    published = artifact_path.read_bytes()
                if isolated_stdout is not stream:
                    isolated_stdout.close()
                self.assertEqual(result, 0)
                self.assertIn(b"RECONCILE PASS", published)
                self.assertNotIn(b"RECONCILE FAIL", published)
                self.assertEqual(artifact_path.read_bytes(), published)

    def test_broken_stdout_pipe_does_not_change_subprocess_success_status(self):
        import time
        import tempfile
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            account = Path(root) / "account.json"
            ready = Path(root) / "ready"
            release = Path(root) / "release"
            snapshot = Path(root) / "published-before-shutdown"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [], "balance_micro": 0, "available_micro": 0,
                "held_micro": 0, "refund_locked_micro": 0}}}}))
            child = """
import os, sys, time
sys.path.insert(0, sys.argv[1])
import reconcile
out, account, ready, release, snapshot = sys.argv[2:]
reconcile.lago_wallets = lambda: []
sys.argv = ['reconcile.py', '--output-dir', out]
open(ready, 'w').close()
while not os.path.exists(release):
    time.sleep(0.005)
status = reconcile.main()
with open(os.path.join(out, 'reconcile-output.txt'), 'rb') as source:
    with open(snapshot, 'wb') as target:
        target.write(source.read())
raise SystemExit(status)
"""
            env = dict(os.environ, WK_ACCOUNT_JSON=str(account))
            proc = subprocess.Popen([sys.executable, "-c", child, str(HERE),
                                     str(out), str(account), str(ready), str(release), str(snapshot)],
                                    cwd=str(HERE), env=env,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                deadline = time.monotonic() + 10
                while not ready.exists() and time.monotonic() < deadline:
                    if proc.poll() is not None:
                        break
                    time.sleep(0.005)
                self.assertTrue(ready.exists(), "subprocess did not reach pipe-close barrier")
                proc.stdout.close()
                release.touch()
                status = proc.wait(timeout=10)
                stderr = proc.stderr.read().decode(errors="replace")
            finally:
                if proc.poll() is None:
                    proc.kill()
                    proc.wait()
                proc.stderr.close()
                if not proc.stdout.closed:
                    proc.stdout.close()
            self.assertEqual(status, 0, stderr)
            artifact_path = out / "reconcile-output.txt"
            artifact = artifact_path.read_bytes()
            self.assertIn(b"RECONCILE PASS", artifact)
            self.assertEqual(artifact, snapshot.read_bytes())

    def test_nonbroken_stdout_flush_error_does_not_change_subprocess_status(self):
        import time
        import tempfile
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            account = Path(root) / "account.json"
            ready = Path(root) / "ready"
            release = Path(root) / "release"
            snapshot = Path(root) / "published-before-shutdown"
            account.write_text(json.dumps({"data": {"benefits": {"credits": {
                "batches": [], "balance_micro": 0, "available_micro": 0,
                "held_micro": 0, "refund_locked_micro": 0}}}}))
            child = """
import os, sys, time
sys.path.insert(0, sys.argv[1])
import reconcile
out, account, ready, release, snapshot = sys.argv[2:]
reconcile.lago_wallets = lambda: []
sys.argv = ['reconcile.py', '--output-dir', out]
original_stdout = sys.stdout
class FlushErrorStream:
    def write(self, value):
        return original_stdout.write(value)
    def flush(self):
        raise OSError('injected ordinary flush error')
    def fileno(self):
        return original_stdout.fileno()
    @property
    def encoding(self):
        return original_stdout.encoding
sys.stdout = FlushErrorStream()
open(ready, 'w').close()
while not os.path.exists(release):
    time.sleep(0.005)
status = reconcile.main()
with open(os.path.join(out, 'reconcile-output.txt'), 'rb') as source:
    with open(snapshot, 'wb') as target:
        target.write(source.read())
raise SystemExit(status)
"""
            env = dict(os.environ, WK_ACCOUNT_JSON=str(account))
            proc = subprocess.Popen([sys.executable, "-c", child, str(HERE),
                                     str(out), str(account), str(ready), str(release), str(snapshot)],
                                    cwd=str(HERE), env=env,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                deadline = time.monotonic() + 10
                while not ready.exists() and time.monotonic() < deadline:
                    if proc.poll() is not None:
                        break
                    time.sleep(0.005)
                self.assertTrue(ready.exists(), "subprocess did not reach pipe-close barrier")
                proc.stdout.close()
                release.touch()
                status = proc.wait(timeout=10)
                stderr = proc.stderr.read().decode(errors="replace")
            finally:
                if proc.poll() is None:
                    proc.kill()
                    proc.wait()
                proc.stderr.close()
                if not proc.stdout.closed:
                    proc.stdout.close()
            self.assertEqual(status, 0, stderr)
            artifact = (out / "reconcile-output.txt").read_bytes()
            self.assertIn(b"RECONCILE PASS", artifact)
            self.assertEqual(artifact, snapshot.read_bytes())

    def test_stdout_fd2_failure_does_not_redirect_stderr_sink(self):
        import tempfile
        original_stdout = sys.stdout
        saved_stderr_fd = os.dup(2)
        isolated_stdout = None
        with tempfile.TemporaryFile(mode="w+b") as sink:
            class Fd2FlushErrorStream:
                def write(self, value):
                    return len(value)
                def flush(self):
                    raise OSError("fd2 wrapper flush error")
                def fileno(self):
                    return 2

            try:
                os.dup2(sink.fileno(), 2)
                sys.stdout = Fd2FlushErrorStream()
                reconcile._emit_text("fd2 descriptor isolation", sys.stdout)
                isolated_stdout = sys.stdout
                os.write(2, b"fd2-marker")
                sink.flush()
                os.lseek(sink.fileno(), 0, os.SEEK_SET)
                self.assertIn(b"fd2-marker", sink.read())
            finally:
                sys.stdout = original_stdout
                if isolated_stdout is not None and isolated_stdout is not original_stdout:
                    isolated_stdout.close()
                os.dup2(saved_stderr_fd, 2)
                os.close(saved_stderr_fd)

    def test_reconcile_main_writes_redacted_fail_for_input_and_fetch_errors(self):
        for source, contents, fetch_error, failed_stage in (
            (None, None, None, "account_input"),
            ("bad.json", "{", None, "account_input"),
            (None, None, "pagination LAGO_SENTINEL", "lago_fetch"),
        ):
            with self.subTest(failed_stage=failed_stage), tempfile.TemporaryDirectory() as root:
                out = Path(root) / "run"
                source_path = Path(root) / "account.json"
                env = {"WK_ACCOUNT_JSON": str(source_path)}
                if contents is not None:
                    source_path.write_text(contents)
                elif failed_stage == "lago_fetch":
                    source_path.write_text(json.dumps({"data": {"benefits": {"credits": {
                        "batches": [], "balance_micro": 0, "available_micro": 0,
                        "held_micro": 0, "refund_locked_micro": 0}}}}))
                else:
                    env["WK_ACCOUNT_JSON"] = str(source_path)
                capture = __import__("io").StringIO()
                with mock.patch.dict(os.environ, env), \
                     mock.patch.object(reconcile, "lago_wallets", side_effect=RuntimeError(fetch_error) if fetch_error else AssertionError("unexpected fetch")), \
                     mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                     mock.patch.object(sys, "stderr", capture):
                    result = reconcile.main()
                self.assertNotEqual(result, 0)
                artifact = (out / "reconcile-output.txt").read_text()
                self.assertIn("RECONCILE FAIL", artifact)
                self.assertIn(failed_stage, artifact)
                self.assertLessEqual(len(artifact), 600)
                self.assertNotIn("LAGO_SENTINEL", artifact)
                self.assertNotIn("LAGO_SENTINEL", capture.getvalue())

    def test_reconcile_main_artifact_writer_failure_stays_nonzero_and_redacted(self):
        import tempfile
        with tempfile.TemporaryDirectory() as root:
            out = Path(root) / "run"
            account = Path(root) / "account.json"
            account.write_text("{")
            env = {"WK_ACCOUNT_JSON": str(account), "LAGO_API_KEY": "LAGO_SENTINEL"}
            capture = __import__("io").StringIO()
            with mock.patch.dict(os.environ, env), \
                 mock.patch.object(tempfile, "NamedTemporaryFile", side_effect=OSError("LAGO_SENTINEL response body")), \
                 mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]), \
                 mock.patch.object(sys, "stderr", capture):
                result = reconcile.main()
            self.assertNotEqual(result, 0)
            diagnostic = capture.getvalue()
            self.assertTrue(diagnostic.startswith("RECONCILE FAIL: artifact write failed ("))
            reason = diagnostic.removeprefix("RECONCILE FAIL: artifact write failed (").removesuffix(")\n")
            self.assertLessEqual(len(reason), 80)
            self.assertLessEqual(len(diagnostic), 240)
            self.assertNotIn("LAGO_SENTINEL", capture.getvalue())
            self.assertNotIn("response body", capture.getvalue())

    def test_reconcile_main_output_preflight_errors_are_bounded(self):
        for existing in (True, False):
            with self.subTest(existing=existing), tempfile.TemporaryDirectory() as root:
                root_path = Path(root)
                output = root_path / "existing" if existing else root_path / "missing-parent" / "run"
                sentinel = output / "sentinel-secret.txt"
                if existing:
                    output.mkdir()
                    sentinel.write_text("sentinel payload\n")
                stderr = __import__("io").StringIO()
                with mock.patch.object(sys, "argv", ["runner", "--output-dir", str(output)]), \
                     mock.patch.object(sys, "stderr", stderr):
                    result = reconcile.main()
                diagnostic = stderr.getvalue()
                self.assertEqual(result, 1)
                self.assertTrue(diagnostic.startswith("RECONCILE FAIL: stage=output_preflight"))
                self.assertLessEqual(len(diagnostic), 240)
                self.assertIn("FileExistsError" if existing else "FileNotFoundError", diagnostic)
                self.assertNotIn(str(output), diagnostic)
                self.assertNotIn("sentinel-secret", diagnostic)
                self.assertNotIn("Traceback", diagnostic)
                if existing:
                    self.assertEqual(sentinel.read_text(), "sentinel payload\n")
                    self.assertEqual(sorted(p.name for p in output.iterdir()), [sentinel.name])
                else:
                    self.assertFalse(output.parent.exists())

    def test_reconcile_main_preserves_argparse_system_exit(self):
        help_output = __import__("io").StringIO()
        with mock.patch.object(sys, "argv", ["runner"]), self.assertRaises(SystemExit) as missing:
            reconcile.main()
        self.assertEqual(missing.exception.code, 2)
        with mock.patch.object(sys, "argv", ["runner", "--help"]), \
             mock.patch.object(sys, "stdout", help_output), self.assertRaises(SystemExit) as help_exit:
            reconcile.main()
        self.assertEqual(help_exit.exception.code, 0)
        self.assertIn("usage:", help_output.getvalue())
        self.assertIn("--output-dir", help_output.getvalue())


    def test_reconcile_main_normal_fail_and_pass_persist(self):
        for active, page, expected in (([], [self.topup_page()], "RECONCILE FAIL"),
                                       ([], [], "RECONCILE PASS")):
            with tempfile.TemporaryDirectory() as root:
                out = Path(root) / "run"
                account = Path(root) / "account.json"
                account.write_text(json.dumps({"data": {"benefits": {"credits": {
                    "batches": page, "balance_micro": sum(r["balance_micro"] for r in page),
                    "available_micro": sum(r["balance_micro"] for r in page),
                    "held_micro": 0, "refund_locked_micro": 0}}}}))
                with mock.patch.dict(os.environ, {"WK_ACCOUNT_JSON": str(account)}), \
                     mock.patch.object(reconcile, "lago_wallets", return_value=active), \
                     mock.patch.object(sys, "argv", ["runner", "--output-dir", str(out)]):
                    result = reconcile.main()
                artifact = (out / "reconcile-output.txt").read_text()
                self.assertIn(expected, artifact)
                self.assertEqual(result, 0 if "PASS" in expected else 1)
