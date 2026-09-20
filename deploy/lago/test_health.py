import json
import tempfile
import unittest
from pathlib import Path

from deploy.lago.health import (
    build_snapshot,
    classify_service,
    overall_status,
    load_release_identity,
)


class ServiceClassificationTests(unittest.TestCase):
    def test_classifies_compose_health_and_liveness_states(self):
        cases = [
            ({"Service": "api", "State": "running", "Health": "healthy"}, "healthy"),
            ({"Service": "api", "State": "running", "Health": "unhealthy"}, "unhealthy"),
            ({"Service": "api", "State": "running", "Health": "starting"}, "starting"),
            ({"Service": "api", "State": "exited", "Health": "healthy"}, "exited"),
            ({"Service": "api-clock", "State": "running"}, "running_unverified"),
            ({"Service": "api-clock", "State": "exited"}, "exited"),
            ({"Service": "api"}, "unknown"),
        ]
        for row, expected_state in cases:
            with self.subTest(row=row):
                self.assertEqual(classify_service(row)["state"], expected_state)

    def test_classification_preserves_image_identity(self):
        row = {
            "Service": "api",
            "State": "running",
            "Health": "healthy",
            "Image": "getlago/lago:v1.53.0@sha256:locked",
            "ImageID": "sha256:local-image-id",
        }

        self.assertEqual(
            classify_service(row),
            {
                "service": "api",
                "state": "healthy",
                "image": "getlago/lago:v1.53.0@sha256:locked",
                "image_id": "sha256:local-image-id",
            },
        )

    def test_missing_service_is_unknown(self):
        self.assertEqual(classify_service(None), {
            "service": "unknown", "state": "unknown", "image": None, "image_id": None
        })


class OverallStatusTests(unittest.TestCase):
    def test_dead_worker_degrades_an_otherwise_healthy_api(self):
        services = {
            "api": {"Health": "healthy", "State": "running"},
            "api-worker": {"Health": "unhealthy", "State": "running"},
            "api-clock": {"State": "running"},
            "db": {"Health": "healthy", "State": "running"},
            "redis": {"Health": "healthy", "State": "running"},
        }

        self.assertEqual(overall_status(api_ok=True, services=services), "degraded")

    def test_core_health_and_running_clock_are_ready(self):
        services = {
            "api": {"Health": "healthy", "State": "running"},
            "api-worker": {"Health": "healthy", "State": "running"},
            "api-clock": {"State": "running"},
            "db": {"Health": "healthy", "State": "running"},
            "redis": {"Health": "healthy", "State": "running"},
        }

        self.assertEqual(overall_status(api_ok=True, services=services), "ready")

    def test_api_failure_or_missing_core_service_is_unavailable(self):
        healthy = {
            "api": {"Health": "healthy", "State": "running"},
            "api-worker": {"Health": "healthy", "State": "running"},
            "api-clock": {"State": "running"},
            "db": {"Health": "healthy", "State": "running"},
            "redis": {"Health": "healthy", "State": "running"},
        }
        self.assertEqual(overall_status(api_ok=False, services=healthy), "unavailable")
        del healthy["db"]
        self.assertEqual(overall_status(api_ok=True, services=healthy), "unavailable")

    def test_stopped_clock_degrades_but_running_clock_without_probe_is_ready(self):
        services = {
            "api": {"Health": "healthy", "State": "running"},
            "api-worker": {"Health": "healthy", "State": "running"},
            "api-clock": {"State": "exited"},
            "db": {"Health": "healthy", "State": "running"},
            "redis": {"Health": "healthy", "State": "running"},
        }

        self.assertEqual(overall_status(api_ok=True, services=services), "degraded")


class ReleaseIdentityTests(unittest.TestCase):
    def test_release_and_digests_are_loaded_from_lock_not_api_text(self):
        lock = {
            "release": "v1.53.0",
            "images": {
                "api": {"repository": "getlago/lago", "tag": "v1.53.0", "digest": "sha256:locked"},
                "front": {"repository": "getlago/lago-front", "tag": "v1.53.0", "digest": "sha256:front"},
            },
        }
        with tempfile.TemporaryDirectory() as temp_dir:
            lock_path = Path(temp_dir) / "images.lock.json"
            lock_path.write_text(json.dumps(lock), encoding="utf-8")

            identity = load_release_identity(lock_path)
            snapshot = build_snapshot([], True, identity)

        self.assertEqual(snapshot["release"], "v1.53.0")
        self.assertEqual(snapshot["images"]["api"]["digest"], "sha256:locked")
        self.assertNotIn("user-controlled release", json.dumps(snapshot))


if __name__ == "__main__":
    unittest.main()
