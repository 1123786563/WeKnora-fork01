#!/usr/bin/env bash
set -euo pipefail

# Validate the files downloaded by the release job before publishing them.
# The React candidate is intentionally separate from the current Vue Lite
# artifact until T25 is accepted, but it must still be structurally valid.

ARTIFACT_DIR="${1:-.}"
cd "${ARTIFACT_DIR}"
shopt -s nullglob

require_matches() {
  local pattern="$1"
  local matches=( ${pattern} )
  if ((${#matches[@]} == 0)); then
    echo "missing release artifact: ${pattern}" >&2
    return 1
  fi
  printf 'release artifact: %s\n' "${matches[@]}"
}

require_matches 'WeKnora-lite_*.tar.gz'
require_matches 'WeKnora-lite_*.tar.gz.sha256'
for platform in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  require_matches "WeKnora-lite_*_${platform}.tar.gz"
  require_matches "WeKnora-lite_*_${platform}.tar.gz.sha256"
done
require_matches 'WeKnora-react-web_*.tar.gz'
require_matches 'WeKnora-react-web_*.tar.gz.sha256'
require_matches 'WeKnora-Lite-App_*'
for desktop_artifact in \
  macOS_universal.dmg macOS_amd64.dmg macOS_arm64.dmg \
  linux_amd64.tar.gz windows_amd64_setup.exe; do
  require_matches "WeKnora-Lite-App_*_${desktop_artifact}"
  require_matches "WeKnora-Lite-App_*_${desktop_artifact}.sha256"
done

checksum_files=(
  WeKnora-lite_*.tar.gz.sha256
  WeKnora-react-web_*.tar.gz.sha256
  WeKnora-Lite-App_*.sha256
)
for checksum_file in "${checksum_files[@]}"; do
  shasum -a 256 -c "${checksum_file}"
done

react_archives=( WeKnora-react-web_*.tar.gz )
if ((${#react_archives[@]} != 1)); then
  echo "expected exactly one React candidate archive, found ${#react_archives[@]}" >&2
  exit 1
fi

extract_dir="$(mktemp -d)"
trap 'rm -rf "${extract_dir}"' EXIT
tar -xzf "${react_archives[0]}" -C "${extract_dir}"

build_info="$(find "${extract_dir}" -type f -path '*/web/BUILD_INFO.json' -print -quit)"
if [[ -z "${build_info}" ]]; then
  echo "React candidate is missing web/BUILD_INFO.json" >&2
  exit 1
fi

python3 - "${build_info}" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text())
if data.get("renderer") != "react":
    raise SystemExit(f"React candidate renderer is {data.get('renderer')!r}")
if data.get("entries") != {"web": "index.html", "embed": "embed.html"}:
    raise SystemExit(f"unexpected React candidate entries: {data.get('entries')!r}")
root = path.parent
for relative in ("index.html", "embed.html", "assets", "embed/assets"):
    target = root / relative
    if not target.exists():
        raise SystemExit(f"React candidate is missing {relative}")
PY

echo "release artifact preflight passed"
