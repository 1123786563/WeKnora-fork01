"""Inventory fixed OpenMeter contracts and perform safe, read-only probes."""

import argparse
import hashlib
import json
import re
import urllib.error
import urllib.request
from pathlib import Path


def operation_paths(text: str, prefix: str) -> dict[str, str]:
    """Extract operation IDs from the deliberately fixed OpenAPI YAML shape."""
    if not prefix or not prefix.startswith("/"):
        raise ValueError("missing server prefix")
    result: dict[str, str] = {}
    path = method = None
    saw_paths = False
    for line in text.splitlines():
        if line == "paths:":
            saw_paths = True
            continue
        if re.match(r"^  /", line):
            if not line.rstrip().endswith(":"):
                raise ValueError("unsupported schema structure")
            path = line.strip()[:-1]
            method = None
            continue
        match = re.match(r"^    (get|post|put|patch|delete):$", line)
        if match:
            if path is None:
                raise ValueError("unsupported schema structure")
            method = match[1].upper()
            continue
        match = re.match(r"^      operationId: (.+)$", line)
        if match:
            if path is None or method is None:
                raise ValueError("unsupported schema structure")
            operation_id = match[1]
            if operation_id in result:
                raise ValueError("duplicate operationId")
            result[operation_id] = f"{method} {prefix}{path}"
    if not saw_paths:
        raise ValueError("unsupported schema structure")
    return result


def _inventory_path() -> Path:
    return Path(__file__).resolve().parents[2] / "docs/superpowers/specs/evidence/2026-09-10-saas-billing-interface-inventory.json"


def _probe(url: str) -> dict[str, str]:
    request = urllib.request.Request(url, method="GET")
    try:
        with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(request, timeout=5) as response:
            return {"status": str(response.status)}
    except Exception as exc:  # probe is evidence collection; environment failures are data
        return {"status": "blocked-env", "exception": type(exc).__name__}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--schema", required=True, type=Path)
    parser.add_argument("--prefix", required=True)
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args(argv)
    inventory = json.loads(_inventory_path().read_text(encoding="utf-8"))
    digest = hashlib.sha256(args.schema.read_bytes()).hexdigest()
    source = next(
        (s for s in inventory["sources"] if str(args.schema).replace("\\", "/").endswith(s["repository_path"])),
        None,
    )
    if source is None or digest != source["sha256"]:
        print(json.dumps({"status": "schema-hash-mismatch"}, ensure_ascii=False))
        return 2
    paths = operation_paths(args.schema.read_text(encoding="utf-8"), args.prefix)
    gets = [(op["operation_id"], op["local_path"]) for op in inventory["operations"] if op["method"] == "GET"][:4]
    probes = {op_id: _probe(args.base_url.rstrip("/") + local_path) for op_id, local_path in gets}
    result = {"release": inventory.get("release"), "operations": paths, "probes": probes}
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"release": result["release"], "probes": probes}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
