#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT_DIR}/scripts/validate-release-lite-artifacts.sh"
FIXTURE="$(mktemp -d)"
trap 'rm -rf "${FIXTURE}"' EXIT

mkdir -p "${FIXTURE}/WeKnora-react-web_vtest/web/assets" \
  "${FIXTURE}/WeKnora-react-web_vtest/web/embed/assets"
printf 'web\n' > "${FIXTURE}/WeKnora-react-web_vtest/web/index.html"
printf 'embed\n' > "${FIXTURE}/WeKnora-react-web_vtest/web/embed.html"
printf '%s\n' '{"renderer":"react","version":"vtest","commit":"fixture","entries":{"web":"index.html","embed":"embed.html"}}' \
  > "${FIXTURE}/WeKnora-react-web_vtest/web/BUILD_INFO.json"
tar -czf "${FIXTURE}/WeKnora-react-web_vtest.tar.gz" -C "${FIXTURE}" WeKnora-react-web_vtest
(
  cd "${FIXTURE}"
  shasum -a 256 WeKnora-react-web_vtest.tar.gz > WeKnora-react-web_vtest.tar.gz.sha256
)
touch "${FIXTURE}/WeKnora-lite_vtest_linux_amd64.tar.gz" \
  "${FIXTURE}/WeKnora-lite_vtest_linux_amd64.tar.gz.sha256" \
  "${FIXTURE}/WeKnora-lite_vtest_linux_arm64.tar.gz" \
  "${FIXTURE}/WeKnora-lite_vtest_linux_arm64.tar.gz.sha256" \
  "${FIXTURE}/WeKnora-lite_vtest_darwin_amd64.tar.gz" \
  "${FIXTURE}/WeKnora-lite_vtest_darwin_amd64.tar.gz.sha256" \
  "${FIXTURE}/WeKnora-lite_vtest_darwin_arm64.tar.gz" \
  "${FIXTURE}/WeKnora-lite_vtest_darwin_arm64.tar.gz.sha256"
for desktop_artifact in \
  macOS_universal.dmg macOS_amd64.dmg macOS_arm64.dmg \
  linux_amd64.tar.gz windows_amd64_setup.exe; do
  touch "${FIXTURE}/WeKnora-Lite-App_vtest_${desktop_artifact}" \
    "${FIXTURE}/WeKnora-Lite-App_vtest_${desktop_artifact}.sha256"
done
for archive in "${FIXTURE}"/WeKnora-lite_*.tar.gz "${FIXTURE}"/WeKnora-Lite-App_*; do
  case "${archive}" in
    *.sha256) continue ;;
  esac
  checksum="${archive}.sha256"
  shasum -a 256 "${archive}" > "${checksum}"
done

"${SCRIPT}" "${FIXTURE}" >/dev/null

rm "${FIXTURE}/WeKnora-react-web_vtest.tar.gz.sha256"
if "${SCRIPT}" "${FIXTURE}" >/dev/null 2>&1; then
  echo "expected missing-checksum fixture to fail" >&2
  exit 1
fi

rm "${FIXTURE}/WeKnora-react-web_vtest.tar.gz"
mkdir -p "${FIXTURE}/WeKnora-react-web_vtest/web/assets" \
  "${FIXTURE}/WeKnora-react-web_vtest/web/embed/assets"
printf 'web\n' > "${FIXTURE}/WeKnora-react-web_vtest/web/index.html"
printf 'embed\n' > "${FIXTURE}/WeKnora-react-web_vtest/web/embed.html"
printf '%s\n' '{"renderer":"vue","version":"vtest","commit":"fixture","entries":{"web":"index.html","embed":"embed.html"}}' \
  > "${FIXTURE}/WeKnora-react-web_vtest/web/BUILD_INFO.json"
tar -czf "${FIXTURE}/WeKnora-react-web_vtest.tar.gz" -C "${FIXTURE}" WeKnora-react-web_vtest
(
  cd "${FIXTURE}"
  shasum -a 256 WeKnora-react-web_vtest.tar.gz > WeKnora-react-web_vtest.tar.gz.sha256
)
if "${SCRIPT}" "${FIXTURE}" >/dev/null 2>&1; then
  echo "expected non-React BUILD_INFO fixture to fail" >&2
  exit 1
fi

echo "release artifact preflight tests passed"
