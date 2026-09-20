from __future__ import annotations

import stat
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from native_env import NativeEnvConfig
from orchestrate import prepare_runtime


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


if __name__ == "__main__":
    unittest.main()
