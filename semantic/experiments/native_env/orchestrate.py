"""Prepare, start, and tear down only the native experiment's resources."""
from __future__ import annotations

import argparse
import json
import os
import secrets
import signal
import socket
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path

from native_env import NativeEnvConfig


@dataclass(frozen=True)
class RuntimePaths:
    root: Path
    compose: Path
    env: Path
    metadata: Path
    app_log: Path
    app_pid: Path


def _write_private(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
        handle.write(content)
    os.chmod(path, 0o600)


def paths_for(config: NativeEnvConfig) -> RuntimePaths:
    root = config.artifact_dir / config.run_id
    return RuntimePaths(root, root / "compose.json", root / "runtime.env", root / "metadata.json", root / "server.log", root / "server.pid")


def prepare_runtime(config: NativeEnvConfig) -> RuntimePaths:
    paths = paths_for(config)
    paths.root.mkdir(parents=True, exist_ok=True)
    paths.compose.write_text(json.dumps(config.compose_document(), indent=2) + "\n", encoding="utf-8")
    secret_lines = {
        "NATIVE_POSTGRES_PASSWORD": secrets.token_urlsafe(24),
        "NATIVE_REDIS_PASSWORD": secrets.token_urlsafe(24),
        "NATIVE_NEO4J_PASSWORD": secrets.token_urlsafe(24),
        "NATIVE_JWT_SECRET": secrets.token_urlsafe(48),
        "NATIVE_SYSTEM_AES_KEY": secrets.token_urlsafe(24),
    }
    _write_private(paths.env, "".join(f"{key}={value}\n" for key, value in secret_lines.items()))
    _write_private(paths.metadata, json.dumps({"run_id": config.run_id, "prefix": config.prefix, "app_url": config.app_url, "created_at": int(time.time()), "state": "prepared"}, indent=2) + "\n")
    return paths


def _compose(config: NativeEnvConfig, paths: RuntimePaths, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["docker", "compose", "--project-name", config.prefix, "--env-file", str(paths.env), "--file", str(paths.compose), *args], check=False, text=True, capture_output=True)


def wait_for_tcp(host: str, port: int, timeout_seconds: int = 90) -> None:
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        try:
            with socket.create_connection((host, port), timeout=1):
                return
        except OSError:
            time.sleep(0.5)
    raise TimeoutError(f"{host}:{port} did not accept TCP within {timeout_seconds}s")


def wait_for_postgres(config: NativeEnvConfig, paths: RuntimePaths, timeout_seconds: int = 90) -> None:
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        result = _compose(config, paths, "exec", "-T", "postgres", "pg_isready", "-U", "native", "-d", "native")
        if result.returncode == 0:
            return
        time.sleep(0.5)
    raise TimeoutError("isolated PostgreSQL did not become ready")


def up(config: NativeEnvConfig) -> int:
    paths = prepare_runtime(config)
    result = _compose(config, paths, "up", "--detach")
    (paths.root / "compose-up.log").write_text(result.stdout + result.stderr, encoding="utf-8")
    if result.returncode:
        return result.returncode
    try:
        for port in (config.postgres_port, config.redis_port, config.neo4j_bolt_port):
            wait_for_tcp("127.0.0.1", port)
        wait_for_postgres(config, paths)
    except TimeoutError as error:
        (paths.root / "readiness-error.log").write_text(str(error) + "\n", encoding="utf-8")
        return 1
    env = dict(os.environ)
    secrets_map = dict(line.split("=", 1) for line in paths.env.read_text(encoding="utf-8").splitlines() if "=" in line)
    env.update({
        "SERVER_HOST": "127.0.0.1", "SERVER_PORT": str(url_port(config.app_url)),
        "DB_DRIVER": "postgres", "DB_HOST": "127.0.0.1", "DB_PORT": str(config.postgres_port), "DB_USER": "native", "DB_PASSWORD": secrets_map["NATIVE_POSTGRES_PASSWORD"], "DB_NAME": "native",
        "REDIS_ADDR": f"127.0.0.1:{config.redis_port}", "REDIS_PASSWORD": secrets_map["NATIVE_REDIS_PASSWORD"], "REDIS_DB": "13", "WEKNORA_REDIS_NAMESPACE": config.prefix,
        "NEO4J_ENABLE": "true", "NEO4J_URI": f"neo4j://127.0.0.1:{config.neo4j_bolt_port}", "NEO4J_USERNAME": "neo4j", "NEO4J_PASSWORD": secrets_map["NATIVE_NEO4J_PASSWORD"],
        "OLLAMA_BASE_URL": config.ollama_url, "STORAGE_TYPE": "local", "LOCAL_STORAGE_BASE_DIR": str(paths.root / "storage"), "JWT_SECRET": secrets_map["NATIVE_JWT_SECRET"], "SYSTEM_AES_KEY": secrets_map["NATIVE_SYSTEM_AES_KEY"],
        "DISABLE_REGISTRATION": "false", "WEKNORA_AUTH_DEFAULT_TENANT_MODE": "create_personal", "AUTO_MIGRATE": "true",
        "SSRF_WHITELIST_EXTRA": "127.0.0.1",
    })
    with paths.app_log.open("w", encoding="utf-8") as log:
        app = subprocess.Popen(["go", "run", "./cmd/server"], cwd=Path(__file__).parents[3], stdout=log, stderr=subprocess.STDOUT, env=env, start_new_session=True)
    _write_private(paths.app_pid, f"{app.pid}\n")
    metadata = json.loads(paths.metadata.read_text(encoding="utf-8"))
    metadata.update({"state": "started", "server_pid": app.pid})
    _write_private(paths.metadata, json.dumps(metadata, indent=2) + "\n")
    return 0


def url_port(url: str) -> int:
    from urllib.parse import urlparse
    return urlparse(url).port or 0


def teardown(config: NativeEnvConfig) -> int:
    paths = paths_for(config)
    if not paths.metadata.exists():
        raise FileNotFoundError("no metadata for this run; refusing cleanup")
    metadata = json.loads(paths.metadata.read_text(encoding="utf-8"))
    if metadata.get("prefix") != config.prefix:
        raise ValueError("metadata does not belong to requested run")
    if paths.app_pid.exists():
        try:
            # ``go run`` forks the compiled server.  The process group created
            # by start_new_session owns both processes, so killing only the go
            # parent would leave a server with stale experiment credentials.
            os.killpg(int(paths.app_pid.read_text().strip()), signal.SIGTERM)
        except ProcessLookupError:
            pass
    result = _compose(config, paths, "down", "--volumes", "--remove-orphans")
    (paths.root / "compose-down.log").write_text(result.stdout + result.stderr, encoding="utf-8")
    metadata["state"] = "torn_down" if result.returncode == 0 else "teardown_failed"
    _write_private(paths.metadata, json.dumps(metadata, indent=2) + "\n")
    return result.returncode


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("prepare", "up", "teardown"))
    parser.add_argument("--config", required=True, type=Path)
    args = parser.parse_args()
    config = NativeEnvConfig.from_file(args.config)
    if args.action == "prepare":
        print(paths_for(config).root)
        prepare_runtime(config)
        return 0
    return up(config) if args.action == "up" else teardown(config)


if __name__ == "__main__":
    raise SystemExit(main())
