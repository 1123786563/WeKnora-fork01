#!/usr/bin/env bash
set -euo pipefail

# Keep the release workflow honest about the second input a Wails Lite app
# needs: its React renderer is compiled into the WebView, while the local Go
# reverse proxy also needs the same Web/Embed tree at runtime.
ROOT_DIR="$(cd "$(dirname ${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOW=${ROOT_DIR}/.github/workflows/release-lite.yml"
NSIS=${ROOT_DIR}/cmd/desktop/build/windows/installer/project.nsi"

python3 - ${WORKFLOW}" <<'PY'
from pathlib import Path
import re
import sys

workflow = Path(sys.argv[1]).read_text()
match = re.search(r"(?ms)^  build-desktop-app:\n(.*?)(?=^  # ── 3\. Create GitHub Release)", workflow)
if not match:
    raise SystemExit("build-desktop-app job is missing")
job = match.group(1)
required = (
    "needs: build-react-artifact",
    "name: react-web-dist",
    "name: Prepare React static runtime resources",
    'cp -R "$' + "{ROOT}/web\" web",
    "test -s web/embed.html",
)
missing = [item for item in required if item not in job]
if missing:
    raise SystemExit("desktop workflow is missing: " + ", ".join(missing))
print("release desktop workflow React resource checks passed")
PY

grep -Fq 'SetOutPath "$INSTDIR\web"' ${NSIS}"
grep -Fq 'File /r "..\..\..\..\..\web\*"' ${NSIS}"
printf '%s\n' 'Windows installer React web resource checks passed'
