#!/usr/bin/env python3
"""Fail-closed validation of dynamic framework loads in an iOS app bundle."""
import pathlib
import plistlib
import re
import subprocess
import sys


def fail(message: str) -> None:
    print(message, file=sys.stderr)
    raise SystemExit(1)


def executable_for(bundle: pathlib.Path, kind: str, containment: pathlib.Path) -> pathlib.Path:
    info = bundle / "Info.plist"
    if not info.is_file():
        fail(f"FRAMEWORK_INFO_MISSING: {info}")
    try:
        with info.open("rb") as stream:
            executable = plistlib.load(stream).get("CFBundleExecutable")
    except Exception as error:  # malformed plist must fail closed
        fail(f"FRAMEWORK_INFO_INVALID: {info}: {error}")
    if not isinstance(executable, str) or not executable:
        fail(f"FRAMEWORK_EXECUTABLE_MISSING: {bundle}")
    candidate = bundle / executable
    path = candidate.resolve()
    try:
        path.relative_to(containment.resolve())
    except ValueError:
        fail(f"FRAMEWORK_BINARY_OUTSIDE_APP: {bundle.name}: {path}")
    if not path.is_file():
        fail(f"FRAMEWORK_BINARY_MISSING: {bundle.name}: {path}")
    return path


def inspect(binary: pathlib.Path) -> list[str]:
    try:
        result = subprocess.run(["otool", "-L", str(binary)], capture_output=True, text=True)
    except FileNotFoundError:
        fail(f"FRAMEWORK_INSPECTION_FAILED: otool not found while inspecting {binary}")
    if result.returncode:
        fail(f"FRAMEWORK_INSPECTION_FAILED: {binary}: {result.stderr.strip() or result.returncode}")
    return result.stdout.splitlines()[1:]


def main(app_path: str) -> None:
    app = pathlib.Path(app_path).resolve()
    app_binary = executable_for(app, "app", app)
    framework_root = app / "Frameworks"
    frameworks = sorted(framework_root.glob("*.framework"))
    available: dict[str, pathlib.Path] = {}
    for framework in frameworks:
        available[framework.name] = executable_for(framework, "framework", framework_root)
    binaries = [(app.name, app_binary)] + [(framework.name, binary) for framework, binary in
                                           ((f, available[f.name]) for f in frameworks)]
    missing: set[tuple[str, str]] = set()
    for owner, binary in binaries:
        for line in inspect(binary):
            match = re.match(r"\s+@rpath/([^/]+\.framework)/([^\s]+)", line)
            if not match:
                continue
            framework_name, requested_binary = match.groups()
            target = available.get(framework_name)
            if target is None or not target.is_file() or target.name != requested_binary:
                missing.add((owner, framework_name))
    if missing:
        for owner, dependency in sorted(missing):
            fail(f"MISSING_FRAMEWORK_DEPENDENCY: {owner} requires {dependency}")
    print("FRAMEWORK_MODE=source-expo-modules" if not (framework_root / "ExpoModulesWorklets.framework").exists()
          else "FRAMEWORK_MODE=precompiled-expo-modules")
    print("FRAMEWORK_CLOSURE_OK")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        fail("usage: verify-ios-framework-closure.py <app.bundle>")
    main(sys.argv[1])
