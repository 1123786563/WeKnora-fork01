#!/usr/bin/env python3
"""craft-web-toolchain v1 build program — the fixed offline web build.

This program is the ONLY build step of a Craft web artifact (#123 / T04):

* it reads a staged ``content.json`` (authored by the OpenCode
  craft-web-build skill) and renders ``output/index.html`` from the PINNED
  template plus the PINNED provisioned dependencies;
* it performs no network access of any kind — no HTTP, no DNS, no package
  manager, no Git — and refuses any content that references an external
  origin: a page that phones home is a failed build, not a clever one;
* it leaves ``output/build-log.json`` identifying the runtime digest, the
  pinned toolchain digest and the REAL exit status, whatever happened.

Path safety: every directory argument is validated in place (``checked_dir``:
non-empty, no parent-referral ``..`` component, no NUL byte, normalized
absolute form); every file target is a ``safe_path`` result — a pathlib Path
built ONLY from fixed literal layout names under one validated root, with a
containment re-check (``is_relative_to``) before any use. The toolchain lock
may name exactly the fixed file set; no lock-supplied string ever becomes a
path component.

Exit codes: 0 success; 2 toolchain pin violation (missing/changed provisioned
file); 3 invalid staged content; 4 render/output failure. The build log is
written for every termination, including failures.
"""

from __future__ import annotations

import argparse
import hashlib
import html as html_mod
import json
import os
import re
import shutil
import sys
import tempfile
from pathlib import Path

SCHEMA = 1
TOOLCHAIN_NAME = "craft-web-toolchain"
TEMPLATE_NAME = "template.html"
BUILD_PROGRAM_NAME = "build.py"
BUILD_LOG_NAME = "build-log.json"
CONTENT_NAME = "content.json"
LOCK_NAME = "toolchain.lock.json"
STAGING_NAME = ".craft-build-staging"
ASSET_OUTPUT_DIR = "assets"
ENTRY_NAME = "index.html"
# The fixed provisioned dependency set: the lock must list exactly these
# basenames, in this fixed layout, pinned by sha256.
FIXED_DEPENDENCIES = ("craft-web.css", "craft-web.js")

EXIT_OK = 0
EXIT_TOOLCHAIN = 2
EXIT_CONTENT = 3
EXIT_RENDER = 4

# External-origin denylist for staged html fragments. A Craft web artifact
# must render from its own local files under the controlled preview origin.
EXTERNAL_URL_RE = re.compile(r"(https?:)?//[^\s\"'<>`]|://", re.IGNORECASE)
ABSOLUTE_REF_RE = re.compile(r"""(?:src|href|action|poster)\s*=\s*["']?\s*(?:/[a-zA-Z]|[a-zA-Z][a-zA-Z0-9+.-]*:)""", re.IGNORECASE)
ACTIVE_DATA_RE = re.compile(r"data:text/html|javascript:", re.IGNORECASE)
CSS_FETCH_RE = re.compile(r"url\(|@import", re.IGNORECASE)
EMBED_TAG_RE = re.compile(r"<\s*(base|iframe|object|embed|form|script|meta)\b", re.IGNORECASE)
# Active content: inline event handler attributes (onclick=, onload=, ...)
# execute attacker script even when every URL check passes (for example the
# classic <img src=x onerror=...>, whose src does not match the absolute
# reference pattern at all), and svg-embedded script bodies. Staged
# fragments are untrusted agent output, so script execution of any shape is
# refused, not just network egress.
EVENT_ATTR_RE = re.compile(r"""\bon[a-z]+\s*=""", re.IGNORECASE)


class BuildError(Exception):
    def __init__(self, exit_code: int, message: str) -> None:
        super().__init__(message)
        self.exit_code = exit_code
        self.message = message


def checked_dir(raw: str, what: str, exit_code: int) -> Path:
    """Validate one directory argument in the function that uses it:
    non-empty, no ``..`` path component, no NUL byte; returns the normalized
    absolute pathlib Path. Refusing parent-referral components keeps every
    derived path beneath the root the caller already trusted."""
    value = raw.strip()
    if not value or "\x00" in value:
        raise BuildError(exit_code, "invalid {} directory: {!r}".format(what, raw))
    for part in re.split(r"[\\/]+", value):
        if part == "..":
            raise BuildError(exit_code, "{} directory must not contain a parent-referral component: {!r}".format(what, raw))
    return Path(os.path.normpath(os.path.abspath(value)))


def safe_path(root: Path, *literal_names: str) -> Path:
    """Validated file path under one validated root.

    Appends ONLY fixed literal layout names (never a lock-supplied string),
    resolves nothing, forbids separators/dot components in the names, and
    re-checks the normalized result is contained in the normalized root
    before returning it. Every file read AND write in this program uses a
    safe_path result.
    """
    root_norm = Path(os.path.normpath(str(root)))
    target = root_norm
    for name in literal_names:
        if not name or name in (".", "..") or "/" in name or "\\" in name or "\x00" in name or os.path.basename(name) != name:
            raise BuildError(EXIT_TOOLCHAIN, "path component refuses the fixed-layout contract: {!r}".format(name))
        target = target / name
    target_norm = Path(os.path.normpath(str(target)))
    if target_norm != root_norm and root_norm not in target_norm.parents:
        raise BuildError(EXIT_TOOLCHAIN, "path escapes its root: {!r}".format(str(target_norm)))
    return target_norm


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 16), b""):
            digest.update(chunk)
    return digest.hexdigest()


def toolchain_digest(template_sha: str, dep_shas: dict, build_sha: str) -> str:
    """Deterministic identity of the pinned toolchain.

    Mirrored byte-for-byte by internal/container CraftWebToolchainDigest:
    sha256("craft-web-toolchain\\0" + template_sha + "\\0" +
    "\\0".join(sorted("name:sha")) + "\\0" + build_sha).
    """
    payload = TOOLCHAIN_NAME.encode() + b"\x00"
    payload += template_sha.encode() + b"\x00"
    payload += b"\x00".join(sorted("{}:{}".format(name, sha).encode() for name, sha in dep_shas.items()))
    payload += b"\x00" + build_sha.encode()
    return hashlib.sha256(payload).hexdigest()


def load_pin(toolchain_dir: str) -> dict:
    """Verify the provisioned toolchain against its lock and derive its digest.

    The lock must name exactly the fixed layout (template.html, build.py and
    the FIXED_DEPENDENCIES under deps/); a lock listing anything else is a
    hostile or corrupted toolchain and refuses the build before any path is
    built from it.
    """
    toolchain_root = checked_dir(toolchain_dir, "toolchain", EXIT_TOOLCHAIN)
    lock_path = safe_path(toolchain_root, LOCK_NAME)
    if not lock_path.is_file():
        raise BuildError(EXIT_TOOLCHAIN, "missing provisioned dependency: {}".format(LOCK_NAME))
    with lock_path.open("r", encoding="utf-8") as handle:
        try:
            lock = json.load(handle)
        except ValueError as malformed:
            raise BuildError(EXIT_TOOLCHAIN, "toolchain lock is not valid JSON: {}".format(malformed))
    if not isinstance(lock, dict):
        raise BuildError(EXIT_TOOLCHAIN, "toolchain lock must be a JSON object")
    if lock.get("name") != TOOLCHAIN_NAME:
        raise BuildError(EXIT_TOOLCHAIN, "toolchain lock names {}".format(lock.get("name")))
    template_pin = lock.get("template")
    program_pin = lock.get("build_program")
    dependencies_pin = lock.get("dependencies")
    if not isinstance(template_pin, dict) or not isinstance(program_pin, dict) or not isinstance(dependencies_pin, list):
        raise BuildError(EXIT_TOOLCHAIN, "toolchain lock template/build_program must be objects and dependencies a list")
    if template_pin.get("name") != TEMPLATE_NAME:
        raise BuildError(EXIT_TOOLCHAIN, "toolchain lock template name must be {}".format(TEMPLATE_NAME))
    if program_pin.get("name") != BUILD_PROGRAM_NAME:
        raise BuildError(EXIT_TOOLCHAIN, "toolchain lock build program name must be {}".format(BUILD_PROGRAM_NAME))

    listed = sorted(dep.get("name") for dep in dependencies_pin if isinstance(dep, dict))
    if listed != sorted(FIXED_DEPENDENCIES) or len(listed) != len(dependencies_pin):
        raise BuildError(EXIT_TOOLCHAIN, "toolchain lock dependency set must be exactly {}: got {}".format(sorted(FIXED_DEPENDENCIES), listed))
    pinned = {dep["name"]: dep["sha256"] for dep in dependencies_pin}

    template_path = safe_path(toolchain_root, TEMPLATE_NAME)
    if not template_path.is_file():
        raise BuildError(EXIT_TOOLCHAIN, "missing provisioned dependency: {}".format(TEMPLATE_NAME))
    template_sha = sha256_file(template_path)
    if template_sha != lock["template"]["sha256"]:
        raise BuildError(EXIT_TOOLCHAIN, "provisioned dependency {} changed (digest mismatch)".format(TEMPLATE_NAME))

    dep_shas = {}
    for dep_name in FIXED_DEPENDENCIES:
        dep_path = safe_path(toolchain_root, "deps", dep_name)
        if not dep_path.is_file():
            raise BuildError(EXIT_TOOLCHAIN, "missing provisioned dependency: deps/{}".format(dep_name))
        actual = sha256_file(dep_path)
        if actual != pinned[dep_name]:
            raise BuildError(EXIT_TOOLCHAIN, "provisioned dependency {} changed (digest mismatch)".format(dep_name))
        dep_shas[dep_name] = actual

    build_path = safe_path(toolchain_root, BUILD_PROGRAM_NAME)
    if not build_path.is_file():
        raise BuildError(EXIT_TOOLCHAIN, "missing provisioned dependency: {}".format(BUILD_PROGRAM_NAME))
    build_sha = sha256_file(build_path)
    if build_sha != lock["build_program"]["sha256"]:
        raise BuildError(EXIT_TOOLCHAIN, "build program changed (digest mismatch)")

    derived = toolchain_digest(template_sha, dep_shas, build_sha)
    if derived != lock["toolchain_digest"]:
        raise BuildError(EXIT_TOOLCHAIN, "toolchain digest does not match the pinned files")

    return {
        "template_version": lock["template"]["version"],
        "template_sha256": template_sha,
        "dependency_shas": dep_shas,
        "build_sha256": build_sha,
        "toolchain_digest": derived,
    }


def pin_from_lock(toolchain_dir: str) -> dict:
    """Identity fields straight from the lock, without file verification.

    Used ONLY on the failure path so a failed build's log still names the
    toolchain identity it attempted (the lock declares what the toolchain
    should be; the failure was the missing/changed bytes). An unreadable or
    hostile lock yields empty identity and the strict consumer rejects the
    log whole — nothing is fabricated.
    """
    try:
        toolchain_root = checked_dir(toolchain_dir, "toolchain", EXIT_TOOLCHAIN)
        lock_path = safe_path(toolchain_root, LOCK_NAME)
        with lock_path.open("r", encoding="utf-8") as handle:
            lock = json.load(handle)
        return {
            "template_version": str(lock.get("template", {}).get("version", "")),
            "template_sha256": str(lock.get("template", {}).get("sha256", "")),
            "toolchain_digest": str(lock.get("toolchain_digest", "")),
        }
    except (BuildError, OSError, ValueError):
        return {"toolchain_digest": "", "template_version": "", "template_sha256": ""}


def render_table(heading: str, table) -> str:
    if not isinstance(table, dict):
        raise BuildError(EXIT_CONTENT, "table section {!r} is not an object".format(heading))
    columns = table.get("columns", [])
    rows = table.get("rows", [])
    if not isinstance(columns, list) or not all(isinstance(c, str) for c in columns):
        raise BuildError(EXIT_CONTENT, "table section {!r} has non-string columns".format(heading))
    if not isinstance(rows, list) or not all(isinstance(r, list) and all(isinstance(c, str) for c in r) for r in rows):
        raise BuildError(EXIT_CONTENT, "table section {!r} has non-string rows".format(heading))
    parts = ["<h2>{}</h2>".format(html_mod.escape(heading))]
    parts.append('<input class="craft-filter" type="search" placeholder="筛选行…" aria-label="筛选表格行">')
    parts.append('<table class="craft-table">')
    parts.append("<thead><tr>{}</tr></thead>".format("".join("<th>{}</th>".format(html_mod.escape(c)) for c in columns)))
    parts.append("<tbody>")
    for row in rows:
        parts.append("<tr>{}</tr>".format("".join("<td>{}</td>".format(html_mod.escape(cell)) for cell in row)))
    parts.append("</tbody></table>")
    return "".join(parts)


def render_html(heading: str, fragment: str) -> str:
    if not isinstance(fragment, str) or not fragment.strip():
        raise BuildError(EXIT_CONTENT, "html section {!r} is empty".format(heading))
    for pattern, why in (
        (EXTERNAL_URL_RE, "external URL"),
        (ABSOLUTE_REF_RE, "absolute or scheme reference"),
        (ACTIVE_DATA_RE, "active data/javascript URI"),
        (CSS_FETCH_RE, "css url()/@import fetch"),
        (EMBED_TAG_RE, "embedding/script/navigation tag"),
        (EVENT_ATTR_RE, "inline event handler attribute"),
    ):
        if pattern.search(fragment):
            raise BuildError(EXIT_CONTENT, "html section {!r} contains a {}: offline local assets only".format(heading, why))
    return "<h2>{}</h2>{}".format(html_mod.escape(heading), fragment)


def load_content(input_dir: str) -> dict:
    input_root = checked_dir(input_dir, "staged material", EXIT_CONTENT)
    content_path = safe_path(input_root, CONTENT_NAME)
    if not content_path.is_file():
        raise BuildError(EXIT_CONTENT, "staged material has no {}".format(CONTENT_NAME))
    with content_path.open("r", encoding="utf-8") as handle:
        try:
            content = json.load(handle)
        except ValueError as malformed:
            raise BuildError(EXIT_CONTENT, "staged {} is not valid JSON: {}".format(CONTENT_NAME, malformed))
    if not isinstance(content, dict):
        raise BuildError(EXIT_CONTENT, "staged {} must be a JSON object".format(CONTENT_NAME))
    title = content.get("title")
    if not isinstance(title, str) or not title.strip():
        raise BuildError(EXIT_CONTENT, "content.json requires a non-empty title")
    for key in ("subtitle", "lang"):
        if key in content and not isinstance(content[key], str):
            raise BuildError(EXIT_CONTENT, "content.json {} must be a string".format(key))
    sections = content.get("sections")
    if not isinstance(sections, list) or not sections:
        raise BuildError(EXIT_CONTENT, "content.json requires a non-empty sections list")
    rendered = []
    for section in sections:
        if not isinstance(section, dict):
            raise BuildError(EXIT_CONTENT, "every section must be an object")
        heading = section.get("heading")
        if not isinstance(heading, str) or not heading.strip():
            raise BuildError(EXIT_CONTENT, "every section requires a heading")
        if "table" in section and "html" not in section:
            rendered.append(render_table(heading, section["table"]))
        elif "html" in section and "table" not in section:
            rendered.append(render_html(heading, section["html"]))
        else:
            raise BuildError(EXIT_CONTENT, "section {!r} must carry exactly one of table/html".format(heading))
    content["rendered_sections"] = "\n".join(rendered)
    return content


PLACEHOLDER_RE = re.compile(r"\{\{(CRAFT_[A-Z0-9_]+)\}\}")


def render_entry(template: str, content: dict, template_version: str) -> str:
    replacements = {
        "CRAFT_LANG": html_mod.escape(content.get("lang") or "zh-CN"),
        "CRAFT_TITLE": html_mod.escape(content["title"]),
        "CRAFT_SUBTITLE": html_mod.escape(content.get("subtitle") or ""),
        "CRAFT_CONTENT": content["rendered_sections"],
        "CRAFT_TEMPLATE_VERSION": html_mod.escape(template_version),
    }

    # Single pass: re.sub never rescans substituted text, so user-controlled
    # title/subtitle/lang can neither re-expand CRAFT_CONTENT into the head
    # nor smuggle a second-order placeholder. Unknown template placeholders
    # are a real template defect; literal {{...}} inside staged content is
    # inert and passes through untouched.
    def substitute(match: "re.Match[str]") -> str:
        key = match.group(1)
        if key not in replacements:
            raise BuildError(EXIT_RENDER, "template references unknown placeholder {}".format(key))
        return replacements[key]

    return PLACEHOLDER_RE.sub(substitute, template)


def write_build_log(output_dir: str, pin: dict, runtime_digest: str, exit_code: int, assets: list, error: str) -> Path:
    output_root = checked_dir(output_dir, "output", exit_code)
    log = {
        "schema": SCHEMA,
        "kind": "web",
        "runtime_digest": runtime_digest,
        "toolchain_digest": pin["toolchain_digest"],
        "template_version": pin["template_version"],
        "template_sha256": pin["template_sha256"],
        "exit_code": exit_code,
        "entry": ENTRY_NAME,
        "assets": assets,
        "egress": "denied",
        "error": error,
    }
    output_root.mkdir(parents=True, exist_ok=True)
    log_path = safe_path(output_root, BUILD_LOG_NAME)
    log_path.write_text(json.dumps(log, ensure_ascii=False, sort_keys=True) + "\n", encoding="utf-8")
    return log_path


def build(toolchain_dir: str, input_dir: str, output_dir: str, runtime_digest: str) -> int:
    toolchain_root = checked_dir(toolchain_dir, "toolchain", EXIT_TOOLCHAIN)
    input_root = checked_dir(input_dir, "staged material", EXIT_CONTENT)
    output_root = checked_dir(output_dir, "output", EXIT_RENDER)
    if not runtime_digest or not runtime_digest.strip():
        raise BuildError(EXIT_CONTENT, "the runtime digest is required (the platform passes the workspace runtime identity)")
    pin = load_pin(str(toolchain_root))
    content = load_content(str(input_root))

    template = safe_path(toolchain_root, TEMPLATE_NAME).read_text(encoding="utf-8")
    entry = render_entry(template, content, pin["template_version"])

    staged = safe_path(output_root, STAGING_NAME)
    if staged.exists():
        shutil.rmtree(staged)
    safe_path(staged, ASSET_OUTPUT_DIR).mkdir(parents=True, exist_ok=True)

    assets = []
    for dep_name in FIXED_DEPENDENCIES:
        shutil.copyfile(str(safe_path(toolchain_root, "deps", dep_name)), str(safe_path(staged, ASSET_OUTPUT_DIR, dep_name)))
        assets.append(ASSET_OUTPUT_DIR + "/" + dep_name)
    safe_path(staged, ENTRY_NAME).write_text(entry + "\n", encoding="utf-8")

    # Atomic-ish publication: the entry and assets land in the output only
    # after every byte rendered, so a failed build never leaves a half entry.
    staged_entry = safe_path(staged, ENTRY_NAME)
    final_entry = safe_path(output_root, ENTRY_NAME)
    final_entry.parent.mkdir(parents=True, exist_ok=True)
    shutil.move(str(staged_entry), str(final_entry))
    for dep_name in FIXED_DEPENDENCIES:
        staged_dep = safe_path(staged, ASSET_OUTPUT_DIR, dep_name)
        final_dep = safe_path(output_root, ASSET_OUTPUT_DIR, dep_name)
        final_dep.parent.mkdir(parents=True, exist_ok=True)
        shutil.move(str(staged_dep), str(final_dep))
    shutil.rmtree(staged, ignore_errors=True)
    write_build_log(str(output_root), pin, runtime_digest, EXIT_OK, assets, "")
    return EXIT_OK


SELFTEST_CONTENT = {
    "title": "offline selftest",
    "subtitle": "pinned toolchain probe",
    "lang": "zh-CN",
    "sections": [
        {"heading": "probe", "table": {"columns": ["k", "v"], "rows": [["build", "offline"]]}},
        {"heading": "note", "html": "<p>local assets only</p>"},
    ],
}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description="fixed offline Craft web build")
    parser.add_argument("--toolchain", default="/opt/craft/web", help="pinned toolchain directory")
    parser.add_argument("--input", help="staged material directory containing content.json")
    parser.add_argument("--output", help="workspace output directory for index.html, assets/ and build-log.json")
    parser.add_argument("--runtime-digest", default="", help="runtime identity the platform stamps into the log")
    parser.add_argument("--selftest", action="store_true", help="build the embedded probe content into a temp directory")
    args = parser.parse_args(argv)

    if args.selftest:
        with tempfile.TemporaryDirectory(prefix="craft-web-selftest-") as tmp:
            input_root = safe_path(Path(tmp), "input")
            output_root = safe_path(Path(tmp), "output")
            input_root.mkdir(parents=True, exist_ok=True)
            safe_path(input_root, CONTENT_NAME).write_text(json.dumps(SELFTEST_CONTENT, ensure_ascii=False), encoding="utf-8")
            code = build(args.toolchain, str(input_root), str(output_root), "sha256:" + "0" * 64)
            if "offline selftest" not in safe_path(output_root, ENTRY_NAME).read_text(encoding="utf-8"):
                print("selftest entry missing", file=sys.stderr)
                return EXIT_RENDER
            print("craft-web-toolchain selftest ok (exit {})".format(code))
            return code

    if not args.input or not args.output:
        parser.error("--input and --output are required (or use --selftest)")

    # Every termination writes the log with the REAL exit status, including
    # the failure paths below. BuildError carries a categorized exit; an
    # OSError during rendering/publication (read-only output, disk full, ...)
    # is still a rendering-environment failure and must not escape as a bare
    # traceback that skips the log.
    try:
        return build(args.toolchain, args.input, args.output, args.runtime_digest)
    except BuildError as failure:
        pin = pin_from_lock(args.toolchain)
        write_build_log(args.output, pin, args.runtime_digest, failure.exit_code, [], failure.message)
        print("craft web build failed (exit {}): {}".format(failure.exit_code, failure.message), file=sys.stderr)
        return failure.exit_code
    except OSError as io_failure:
        pin = pin_from_lock(args.toolchain)
        message = "render/publish IO failure: {}".format(io_failure)
        write_build_log(args.output, pin, args.runtime_digest, EXIT_RENDER, [], message)
        print("craft web build failed (exit {}): {}".format(EXIT_RENDER, message), file=sys.stderr)
        return EXIT_RENDER


if __name__ == "__main__":
    try:
        sys.exit(main())
    except BuildError as fatal:
        print("craft web build failed (exit {}): {}".format(fatal.exit_code, fatal.message), file=sys.stderr)
        sys.exit(fatal.exit_code)
    except OSError as fatal_io:
        # The output directory itself is unusable, so even the log cannot be
        # written; surface the real cause instead of a bare traceback.
        print("craft web build failed (exit {}): unloggable IO failure: {}".format(EXIT_RENDER, fatal_io), file=sys.stderr)
        sys.exit(EXIT_RENDER)
