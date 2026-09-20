from __future__ import annotations

import stat
import os
import subprocess
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from native_env import NativeEnvConfig
from orchestrate import _process_identity, ensure_not_live_run, paths_for, prepare_runtime, teardown


class PrepareRuntimeTest(unittest.TestCase):
    def test_creates_only_run_scoped_compose_and_private_runtime_env(self) -> None:
        with TemporaryDirectory() as directory:
            config = NativeEnvConfig.from_mapping({"run_id": "unit-a", "artifact_dir": directory})
            paths = prepare_runtime(config)
            self.assertIn("semantica-v03-native-unit-a", paths.compose.read_text())
            self.assertEqual(stat.S_IMODE(paths.env.stat().st_mode), 0o600)
            self.assertEqual(stat.S_IMODE(paths.metadata.stat().st_mode), 0o600)
            self.assertNotIn("NATIVE_POSTGRES_PASSWORD", paths.metadata.read_text())
            values = dict(line.split("=", 1) for line in paths.env.read_text().splitlines())
            self.assertEqual(len(values["NATIVE_SYSTEM_AES_KEY"].encode("utf-8")), 32)

    def test_rejects_repeated_up_for_a_live_run_without_overwriting_metadata(self) -> None:
        with TemporaryDirectory() as directory:
            config = NativeEnvConfig.from_mapping({"run_id": "unit-live", "artifact_dir": directory})
            paths = prepare_runtime(config)
            original = paths.metadata.read_text()
            paths.metadata.write_text('{"run_id":"unit-live","prefix":"semantica-v03-native-unit-live","state":"started","server_pid":123}\n')
            with patch("orchestrate._process_identity", return_value={"pid": 123, "pgid": 123, "started": "then", "command": "go run ./cmd/server", "nonce": "n"}):
                with self.assertRaisesRegex(RuntimeError, "already live"):
                    ensure_not_live_run(config)
            self.assertNotEqual(paths.metadata.read_text(), original)

    def test_teardown_refuses_to_signal_a_stale_process_identity(self) -> None:
        with TemporaryDirectory() as directory:
            config = NativeEnvConfig.from_mapping({"run_id": "unit-stale", "artifact_dir": directory})
            paths = prepare_runtime(config)
            paths.app_pid.write_text("123\n")
            paths.metadata.write_text('{"run_id":"unit-stale","prefix":"semantica-v03-native-unit-stale","state":"started","server_pid":123,"process":{"pid":123,"pgid":123,"started":"old","command":"go run ./cmd/server","nonce":"n"}}\n')
            with patch("orchestrate._process_identity", return_value={"pid": 123, "pgid": 123, "started": "new", "command": "go run ./cmd/server", "nonce": "n"}), patch("orchestrate._compose") as compose, patch("orchestrate.os.killpg") as killpg:
                compose.return_value.returncode = 0
                compose.return_value.stdout = ""
                compose.return_value.stderr = ""
                self.assertEqual(teardown(config), 0)
            killpg.assert_not_called()

    def test_process_identity_reads_nonce_from_run_scoped_launcher(self) -> None:
        with TemporaryDirectory() as directory:
            nonce = "a" * 48
            launcher = Path(directory) / f"native-go-{nonce}"
            os.symlink("/bin/sleep", launcher)
            process = subprocess.Popen([str(launcher), "5"])
            try:
                identity = _process_identity(process.pid)
            finally:
                process.terminate()
                process.wait()
            self.assertIsNotNone(identity)
            self.assertEqual(identity["nonce"], nonce)


if __name__ == "__main__":
    unittest.main()
