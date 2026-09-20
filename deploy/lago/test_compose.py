"""Static contract tests for the pinned Lago Community Compose environment.

These assertions are pure text/schema checks: they need no Docker daemon and
verify the deployment files that ship in the repository.
"""

import json
import re
import unittest
from pathlib import Path


DEPLOY_DIR = Path(__file__).resolve().parent
COMPOSE_PATH = DEPLOY_DIR / "compose.yaml"
LOCK_PATH = DEPLOY_DIR / "images.lock.json"

REQUIRED_SERVICES = {"api", "api-worker", "api-clock", "db", "redis", "front", "pdf"}


def load_compose():
    return COMPOSE_PATH.read_text(encoding="utf-8")


def load_lock():
    return json.loads(LOCK_PATH.read_text(encoding="utf-8"))


def service_blocks(compose_text):
    """Map each service key under `services:` to its raw indented block."""
    blocks = {}
    current = None
    in_services = False
    for line in compose_text.splitlines():
        if line.strip() == "services:" and not line.startswith((" ", "#")):
            in_services = True
            current = None
            continue
        if in_services and not line.startswith((" ", "#")) and line.strip():
            in_services = False
            continue
        if not in_services:
            continue
        match = re.match(r"^  ([A-Za-z0-9_.-]+):\s*$", line)
        if match:
            current = match.group(1)
            blocks[current] = []
        elif current is not None and line.startswith(" "):
            blocks[current].append(line)
    return {name: "\n".join(lines) for name, lines in blocks.items()}


def published_port_lines(compose_text):
    """Return every host-published port entry found inside `ports:` blocks."""
    published = []
    in_ports = False
    for line in compose_text.splitlines():
        stripped = line.strip()
        if stripped == "ports:":
            in_ports = True
            continue
        if in_ports:
            if stripped.startswith("- "):
                published.append(stripped[2:].strip())
            elif stripped:
                in_ports = False
    return published


def top_level_volume_names(compose_text):
    names = set()
    in_volumes = False
    for line in compose_text.splitlines():
        if line.strip() == "volumes:" and not line.startswith((" ", "#")):
            in_volumes = True
            continue
        if in_volumes and not line.startswith((" ", "#")) and line.strip():
            in_volumes = False
            continue
        if not in_volumes:
            continue
        match = re.match(r"^  ([A-Za-z0-9_.-]+):\s*$", line)
        if match:
            names.add(match.group(1))
    return names


class ImageLockTests(unittest.TestCase):
    def test_lock_pins_release_and_exactly_five_image_roles(self):
        lock = load_lock()
        self.assertEqual(lock["release"], "v1.53.0")
        self.assertEqual(
            set(lock["images"]),
            {"api", "front", "db", "redis", "pdf"},
        )

    def test_lock_records_repository_tag_digest_platform_and_date(self):
        lock = load_lock()
        for role, image in lock["images"].items():
            for field in ("repository", "tag", "digest", "platforms", "verified"):
                with self.subTest(role=role, field=field):
                    self.assertIn(field, image)
            self.assertRegex(image["repository"], r"^[a-z0-9._/-]+$")
            self.assertTrue(image["tag"])
            self.assertRegex(image["digest"], r"^sha256:[0-9a-f]{64}$")
            self.assertTrue(image["platforms"])
            for platform in image["platforms"]:
                self.assertRegex(platform, r"^linux/(amd64|arm64)$")
            self.assertRegex(image["verified"], r"^\d{4}-\d{2}-\d{2}$")

    def test_every_runtime_image_uses_its_locked_digest(self):
        lock = load_lock()
        compose = load_compose()
        for image in lock["images"].values():
            with self.subTest(repository=image["repository"]):
                self.assertIn(f'{image["repository"]}@{image["digest"]}', compose)

    def test_every_compose_image_reference_is_locked(self):
        lock = load_lock()
        compose = load_compose()
        locked_refs = {
            f'{image["repository"]}@{image["digest"]}' for image in lock["images"].values()
        }
        referenced = re.findall(r"^\s*image:\s*(\S+)", compose, re.MULTILINE)
        self.assertTrue(referenced)
        for ref in referenced:
            with self.subTest(ref=ref):
                self.assertIn(ref, locked_refs)


class ComposeTopologyTests(unittest.TestCase):
    def test_required_services_are_defined(self):
        blocks = service_blocks(load_compose())
        missing = REQUIRED_SERVICES - set(blocks)
        self.assertEqual(missing, set(), f"missing services: {sorted(missing)}")
        for name in REQUIRED_SERVICES:
            with self.subTest(service=name):
                self.assertTrue(
                    "image:" in blocks[name] or "<<: *" in blocks[name],
                    f"{name} must pin an image directly or via a merge anchor",
                )

    def test_no_fixed_container_names(self):
        self.assertNotIn("container_name", load_compose())

    def test_no_clickhouse_or_kafka_services(self):
        compose = load_compose().lower()
        self.assertNotIn("clickhouse", compose)
        self.assertNotIn("kafka", compose)

    def test_published_ports_are_loopback_only_and_exclude_db_and_redis(self):
        compose = load_compose()
        blocks = service_blocks(compose)
        for name in ("db", "redis"):
            with self.subTest(service=name):
                self.assertNotIn("ports:", blocks[name])

        published = published_port_lines(compose)
        self.assertEqual(len(published), 2, f"unexpected published ports: {published}")
        for entry in published:
            with self.subTest(entry=entry):
                self.assertRegex(
                    entry,
                    r"^\"?127\.0\.0\.1:\$\{LAGO_[A-Z]+_PORT:-\d+\}:\d+\"?$",
                )

    def test_api_and_front_default_to_the_reserved_loopback_ports(self):
        compose = load_compose()
        self.assertIn('"127.0.0.1:${LAGO_API_PORT:-48889}:3000"', compose)
        self.assertIn('"127.0.0.1:${LAGO_FRONT_PORT:-48890}:80"', compose)

    def test_volumes_are_lago_scoped_named_volumes(self):
        compose = load_compose()
        self.assertEqual(
            top_level_volume_names(compose),
            {"lago_postgres_data", "lago_redis_data", "lago_storage_data"},
        )
        self.assertNotIn("external", compose)
        mounts = re.findall(r"^\s+-\s+([A-Za-z0-9_.-]+):/\S+", compose, re.MULTILINE)
        self.assertTrue(mounts)
        for source in mounts:
            with self.subTest(volume=source):
                self.assertTrue(source.startswith("lago_"))

    def test_community_build_keeps_the_license_flag_empty(self):
        self.assertIn('"LAGO_LICENSE": ${LAGO_LICENSE:-}', load_compose())


if __name__ == "__main__":
    unittest.main()
