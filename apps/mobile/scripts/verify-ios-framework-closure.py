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


def executable_for(bundle: pathlib.Path, containment: pathlib.Path) -> pathlib.Path:
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
        relative = path.relative_to(containment.resolve())
    except ValueError:
        fail(f"FRAMEWORK_BINARY_OUTSIDE_BUNDLE: {bundle.name}: {path}")
    if containment.name.endswith(".framework") and relative.parts[0] != executable:
        # Versioned frameworks may declare Foo and resolve to Versions/A/Foo.
        if path.name != executable:
            fail(f"FRAMEWORK_BINARY_OUTSIDE_BUNDLE: {bundle.name}: {path}")
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


def main(app_path: str, properties_path: str) -> None:
    try:
        with pathlib.Path(properties_path).open("rb") as stream:
            properties = plistlib.load(stream) if properties_path.endswith(".plist") else __import__("json").load(stream)
    except Exception as error:
        fail(f"FRAMEWORK_MODE_PROPERTIES_INVALID: {properties_path}: {error}")
    source_value = properties.get("ios.buildReactNativeFromSource")
    expo_value = properties.get("EXPO_USE_PRECOMPILED_MODULES")
    if source_value not in ("true", "false") or expo_value not in ("true", "false"):
        fail("FRAMEWORK_MODE_PROPERTIES_INVALID: expected string ios.buildReactNativeFromSource and EXPO_USE_PRECOMPILED_MODULES values")
    if (source_value, expo_value) not in (("true", "false"), ("false", "true")):
        fail(f"FRAMEWORK_MODE_PROPERTIES_CONTRADICTORY: source={source_value}, precompiled={expo_value}")
    mode = "source-expo-modules" if source_value == "true" else "precompiled-expo-modules"
    app = pathlib.Path(app_path).resolve()
    app_binary = executable_for(app, app)
    framework_root = app / "Frameworks"
    frameworks = sorted(framework_root.glob("*.framework"))
    available: dict[str, pathlib.Path] = {}
    for framework in frameworks:
        available[framework.name] = executable_for(framework, framework)
    if mode == "source-expo-modules" and (framework_root / "ExpoModulesWorklets.framework").exists():
        fail("FRAMEWORK_MODE_MISMATCH: source Expo mode contains ExpoModulesWorklets.framework")
    binaries = [(app.name, app_binary)] + [(framework.name, binary) for framework, binary in
                                           ((f, available[f.name]) for f in frameworks)]
    missing: set[tuple[str, str]] = set()
    for owner, binary in binaries:
        for line in inspect(binary):
            match = re.match(r"\s+@rpath/([^/]+\.framework)/([^\s]+)", line)
            if not match:
                continue
            framework_name, requested_path = match.groups()
            target = available.get(framework_name)
            requested = ((app / "Frameworks" / framework_name / requested_path).resolve()
                         if target is not None else None)
            if target is None or not target.is_file() or requested != target or not requested.is_file():
                missing.add((owner, framework_name))
    if missing:
        for owner, dependency in sorted(missing):
            fail(f"MISSING_FRAMEWORK_DEPENDENCY: {owner} requires {dependency}")
    print(f"FRAMEWORK_MODE={mode}")
    print("FRAMEWORK_CLOSURE_OK")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        fail("usage: verify-ios-framework-closure.py <app.bundle> <generated-Podfile.properties.json>")
    main(sys.argv[1], sys.argv[2])
