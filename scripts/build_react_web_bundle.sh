#!/usr/bin/env bash
set -euo pipefail

# Build the React Web and isolated Embed entries into one Lite-compatible
# candidate directory. Production Lite remains on the Vue artifact until T25
# accepts the migration and switches the release input explicitly.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/dist/react-web"
VERSION="${REACT_BUILD_VERSION:-${VERSION:-dev}}"
COMMIT="${VITE_FRONTEND_COMMIT:-$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || printf unknown)}"

cd "${ROOT_DIR}"
rm -rf "${OUT_DIR}"
pnpm run build:web
pnpm run build:embed

mkdir -p "${OUT_DIR}/web/embed"
cp -R apps/web/dist/. "${OUT_DIR}/web/"
cp apps/embed/dist/index.html "${OUT_DIR}/web/embed.html"
cp -R apps/embed/dist/assets "${OUT_DIR}/web/embed/"

cat > "${OUT_DIR}/web/BUILD_INFO.json" <<EOF
{
  "renderer": "react",
  "version": "${VERSION}",
  "commit": "${COMMIT}",
  "entries": {
    "web": "index.html",
    "embed": "embed.html"
  }
}
EOF

test -s "${OUT_DIR}/web/index.html"
test -s "${OUT_DIR}/web/embed.html"
test -s "${OUT_DIR}/web/BUILD_INFO.json"
printf 'React Web/Embed bundle: %s\n' "${OUT_DIR}/web"
