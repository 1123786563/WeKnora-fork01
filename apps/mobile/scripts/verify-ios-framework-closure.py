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
    # Skip only the exact image header forms emitted by otool -L. A dependency
    # may legitimately share the inspected image's path as a string prefix.
    header = re.compile(re.escape(str(binary)) + r"(?: \(architecture [^)]+\))?:")
    return [line for line in result.stdout.splitlines() if not header.fullmatch(line.strip())]


def main(app_path: str, properties_path: str) -> None:
    try:
        with pathlib.Path(properties_path).open("rb") as stream:
            properties = plistlib.load(stream) if properties_path.endswith(".plist") else __import__("json").load(stream)
    except Exception as error:
        fail(f"FRAMEWORK_MODE_PROPERTIES_INVALID: {properties_path}: {error}")
    if not isinstance(properties, dict):
        fail(f"FRAMEWORK_MODE_PROPERTIES_INVALID: expected object in {properties_path}")
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
    binaries = [(app.name, app_binary)] + [(framework.name, available[framework.name]) for framework in frameworks]
    missing: set[tuple[str, str]] = set()
    for owner, binary in binaries:
        for line in inspect(binary):
            load = line.strip().split(" ", 1)[0]
            if load.startswith(("/System/Library/", "/usr/lib/")):
                continue
            token_match = re.match(r"(?P<token>@(?:rpath|loader_path|executable_path))/(?P<remainder>[^\s]+)$", load)
            component = re.search(r"(?:^|/)(?P<framework>[^/]+\.framework)(?:/|$)", load)
            match = re.search(r"(?:^|/)(?P<framework>[^/]+\.framework)/(?P<requested>[^\s]+)$", token_match.group("remainder")) if token_match else None
            if not token_match or not match:
                framework_name = component.group("framework") if component else load
                if component and (load.endswith(".framework") or load.endswith(".framework/")):
                    fail(f"MALFORMED_FRAMEWORK_LOAD_PATH: {owner} requires {framework_name}: {load}")
                if component:
                    fail(f"UNSUPPORTED_FRAMEWORK_LOAD_PATH: {owner}: {load}")
                fail(f"UNSUPPORTED_DYNAMIC_LIBRARY_LOAD: {owner}: {load}")
            token = token_match.group("token")
            remainder = token_match.group("remainder")
            framework_name = match.group("framework")
            target = available.get(framework_name)
            if token == "@rpath":
                base = app / "Frameworks"
            elif token == "@loader_path":
                base = binary.parent
            else:
                base = app_binary.parent
            requested = (base / remainder).resolve()
            try:
                requested.relative_to(app)
            except ValueError:
                fail(f"FRAMEWORK_LOAD_OUTSIDE_APP: {owner}: {load}")
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
