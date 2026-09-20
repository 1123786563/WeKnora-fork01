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
import urllib.request
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
    app_launcher: Path


def _write_private(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
        handle.write(content)
    os.chmod(path, 0o600)


def paths_for(config: NativeEnvConfig) -> RuntimePaths:
    root = config.artifact_dir / config.run_id
    return RuntimePaths(root, root / "compose.json", root / "runtime.env", root / "metadata.json", root / "server.log", root / "server.pid", root / "native-go-launcher")


def prepare_runtime(config: NativeEnvConfig) -> RuntimePaths:
    ensure_not_live_run(config)
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
    _write_private(paths.metadata, json.dumps({"run_id": config.run_id, "prefix": config.prefix, "app_url": config.app_url, "created_at": int(time.time()), "state": "prepared", "run_nonce": secrets.token_hex(24)}, indent=2) + "\n")
    return paths


def _read_metadata(paths: RuntimePaths) -> dict:
    return json.loads(paths.metadata.read_text(encoding="utf-8"))


def ensure_not_live_run(config: NativeEnvConfig) -> None:
    """Never overwrite ownership state for a run that may still own resources."""
    paths = paths_for(config)
    if not paths.metadata.exists():
        return
    metadata = _read_metadata(paths)
    if metadata.get("run_id") != config.run_id or metadata.get("prefix") != config.prefix:
        raise ValueError("runtime metadata does not belong to requested run")
    if metadata.get("state") != "torn_down":
        raise RuntimeError(f"native run {config.run_id} is already live or needs teardown")


def _process_identity(pid: int) -> dict[str, object] | None:
    """Read identity fields that make a stale PID unsafe to signal."""
    result = subprocess.run(["ps", "-p", str(pid), "-o", "pid=", "-o", "pgid=", "-o", "lstart=", "-o", "command="], check=False, text=True, capture_output=True)
    line = result.stdout.strip()
    if result.returncode or not line:
        return None
    parts = line.split(maxsplit=7)
    if len(parts) != 8:
        return None
    environment = subprocess.run(["ps", "eww", "-p", str(pid), "-o", "command="], check=False, text=True, capture_output=True).stdout
    marker = "NATIVE_ENV_RUN_NONCE="
    nonce = environment.split(marker, 1)[1].split()[0] if marker in environment else None
    if nonce is None:
        import re
        match = re.search(r"native-go-([0-9a-f]{48})", parts[7])
        nonce = match.group(1) if match else None
    return {"pid": int(parts[0]), "pgid": int(parts[1]), "started": " ".join(parts[2:7]), "command": parts[7], "nonce": nonce}


def _matches_recorded_process(metadata: dict) -> bool:
    expected = metadata.get("process")
    if not isinstance(expected, dict) or not isinstance(expected.get("pid"), int):
        return False
    actual = _process_identity(expected["pid"])
    return actual is not None and all(actual.get(key) == expected.get(key) for key in ("pid", "pgid", "started", "command", "nonce"))


def _terminate_recorded_process(metadata: dict, timeout_seconds: int = 10) -> None:
    if not _matches_recorded_process(metadata):
        return
    process = metadata["process"]
    os.killpg(int(process["pgid"]), signal.SIGTERM)
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        if not _matches_recorded_process(metadata):
            return
        time.sleep(0.1)
    if _matches_recorded_process(metadata):
        os.killpg(int(process["pgid"]), signal.SIGKILL)


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
    try:
        metadata = _read_metadata(paths)
        metadata["state"] = "starting"
        _write_private(paths.metadata, json.dumps(metadata, indent=2) + "\n")
        result = _compose(config, paths, "up", "--detach")
        (paths.root / "compose-up.log").write_text(result.stdout + result.stderr, encoding="utf-8")
        if result.returncode:
            teardown(config)
            return result.returncode
        for port in (config.postgres_port, config.redis_port, config.neo4j_bolt_port):
            wait_for_tcp("127.0.0.1", port)
        wait_for_postgres(config, paths)
    except (OSError, TimeoutError) as error:
        (paths.root / "readiness-error.log").write_text(str(error) + "\n", encoding="utf-8")
        teardown(config)
        return 1
    # Do not inherit ambient provider, storage, proxy, or user credentials.
    env = {key: os.environ[key] for key in ("PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "CGO_ENABLED") if key in os.environ}
    secrets_map = dict(line.split("=", 1) for line in paths.env.read_text(encoding="utf-8").splitlines() if "=" in line)
    env.update({
        "SERVER_HOST": "127.0.0.1", "SERVER_PORT": str(url_port(config.app_url)),
        "DB_DRIVER": "postgres", "DB_HOST": "127.0.0.1", "DB_PORT": str(config.postgres_port), "DB_USER": "native", "DB_PASSWORD": secrets_map["NATIVE_POSTGRES_PASSWORD"], "DB_NAME": "native",
        "REDIS_ADDR": f"127.0.0.1:{config.redis_port}", "REDIS_PASSWORD": secrets_map["NATIVE_REDIS_PASSWORD"], "REDIS_DB": "13", "WEKNORA_REDIS_NAMESPACE": config.prefix,
        "NEO4J_ENABLE": "true", "NEO4J_URI": f"neo4j://127.0.0.1:{config.neo4j_bolt_port}", "NEO4J_USERNAME": "neo4j", "NEO4J_PASSWORD": secrets_map["NATIVE_NEO4J_PASSWORD"],
        "OLLAMA_BASE_URL": config.ollama_url, "STORAGE_TYPE": "local", "LOCAL_STORAGE_BASE_DIR": str(paths.root / "storage"), "JWT_SECRET": secrets_map["NATIVE_JWT_SECRET"], "SYSTEM_AES_KEY": secrets_map["NATIVE_SYSTEM_AES_KEY"],
        "DISABLE_REGISTRATION": "false", "WEKNORA_AUTH_DEFAULT_TENANT_MODE": "create_personal", "AUTO_MIGRATE": "true",
        "SSRF_WHITELIST_EXTRA": "127.0.0.1",
        "NATIVE_ENV_RUN_NONCE": metadata["run_nonce"],
        "HTTP_PROXY": "", "HTTPS_PROXY": "", "ALL_PROXY": "", "NO_PROXY": "*",
    })
    with paths.app_log.open("w", encoding="utf-8") as log:
        go_binary = next((Path(entry) / "go" for entry in env["PATH"].split(os.pathsep) if (Path(entry) / "go").is_file()), None)
        if go_binary is None:
            teardown(config)
            return 1
        paths.app_launcher.unlink(missing_ok=True)
        nonce_launcher = paths.root / f"native-go-{metadata['run_nonce']}"
        os.symlink(go_binary, nonce_launcher)
        app = subprocess.Popen([str(nonce_launcher), "run", "./cmd/server"], cwd=Path(__file__).parents[3], stdout=log, stderr=subprocess.STDOUT, env=env, start_new_session=True)
    _write_private(paths.app_pid, f"{app.pid}\n")
    deadline = time.monotonic() + 90
    while time.monotonic() < deadline:
        if app.poll() is not None:
            teardown(config)
            return 1
        try:
            with urllib.request.urlopen(config.app_url + "/health", timeout=1) as response:
                if response.status == 200:
                    break
        except OSError:
            time.sleep(0.5)
    else:
        identity = _process_identity(app.pid)
        if identity is not None:
            metadata["process"] = identity
            _terminate_recorded_process(metadata)
        teardown(config)
        return 1
    identity = _process_identity(app.pid)
    if identity is None or identity.get("nonce") != metadata["run_nonce"]:
        teardown(config)
        return 1
    metadata.update({"state": "started", "server_pid": app.pid, "process": identity})
    _write_private(paths.metadata, json.dumps(metadata, indent=2) + "\n")
    return 0


def url_port(url: str) -> int:
    from urllib.parse import urlparse
    return urlparse(url).port or 0


def teardown(config: NativeEnvConfig) -> int:
    paths = paths_for(config)
    if not paths.metadata.exists():
        raise FileNotFoundError("no metadata for this run; refusing cleanup")
    metadata = _read_metadata(paths)
    if metadata.get("prefix") != config.prefix or metadata.get("run_id") != config.run_id:
        raise ValueError("metadata does not belong to requested run")
    if metadata.get("state") == "torn_down":
        return 0
    _terminate_recorded_process(metadata)
    result = _compose(config, paths, "down", "--volumes", "--remove-orphans")
    (paths.root / "compose-down.log").write_text(result.stdout + result.stderr, encoding="utf-8")
    metadata["state"] = "torn_down" if result.returncode == 0 else "teardown_failed"
    _write_private(paths.metadata, json.dumps(metadata, indent=2) + "\n")
    if result.returncode == 0:
        for launcher in paths.root.glob("native-go-*"):
            launcher.unlink(missing_ok=True)
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
