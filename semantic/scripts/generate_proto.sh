#!/bin/bash
# Single pinned entry point for regenerating the Semantica wire stubs
# (semantic/proto/semantic.proto -> Go + Python). Generators are pinned:
#   protoc: grpcio-tools 1.80.0 (semantic/pyproject.toml dev group)
#   protoc-gen-go: v1.36.11 (matches google.golang.org/protobuf in go.mod)
#     install: go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
#   protoc-gen-go-grpc: v1.5.1
#     install: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
# Regeneration must be idempotent: running twice leaves git diff empty.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROTO_DIR="${REPO_ROOT}/semantic/proto"
PY_OUT="${REPO_ROOT}/semantic/semantic_service/proto"
GO_OUT="${REPO_ROOT}/semantic/proto"

# Locate plugins: PATH first (custom GOBIN), then the default GOPATH bin.
plugin_path() {
  local name="$1"
  if command -v "${name}" >/dev/null 2>&1; then
    command -v "${name}"
    return 0
  fi
  local candidate="${HOME}/go/bin/${name}"
  if [ -x "${candidate}" ]; then
    echo "${candidate}"
    return 0
  fi
  echo "ERROR: ${name} not found in PATH or ${HOME}/go/bin." >&2
  echo "       Install the pinned version, see the header of this script." >&2
  return 1
}

PROTOC_GEN_GO="$(plugin_path protoc-gen-go)"
PROTOC_GEN_GO_GRPC="$(plugin_path protoc-gen-go-grpc)"

mkdir -p "${PY_OUT}"
touch "${PY_OUT}/__init__.py"

# Python stubs (protoc from grpcio-tools, run inside the semantic project env)
(cd "${REPO_ROOT}" && uv run --project semantic python -m grpc_tools.protoc \
  -I "${PROTO_DIR}" \
  --python_out="${PY_OUT}" \
  --pyi_out="${PY_OUT}" \
  --grpc_python_out="${PY_OUT}" \
  "${PROTO_DIR}/semantic.proto")

# grpcio-tools emits a top-level import; make it package-relative. The grep
# proof makes a silent generator-format change fail loudly here instead of
# surfacing later as an ImportError.
if grep -q '^import semantic_pb2 as' "${PY_OUT}/semantic_pb2_grpc.py"; then
  if [ "$(uname)" = "Darwin" ]; then
    sed -i '' 's/^import semantic_pb2 as/from . import semantic_pb2 as/' "${PY_OUT}/semantic_pb2_grpc.py"
  else
    sed -i 's/^import semantic_pb2 as/from . import semantic_pb2 as/' "${PY_OUT}/semantic_pb2_grpc.py"
  fi
else
  echo "ERROR: expected the top-level semantic_pb2 import in semantic_pb2_grpc.py was not found;" >&2
  echo "       grpcio-tools output format changed - update this script deliberately." >&2
  exit 1
fi

# Go stubs through the pinned plugins.
(cd "${REPO_ROOT}" && uv run --project semantic python -m grpc_tools.protoc \
  -I "${PROTO_DIR}" \
  --plugin="protoc-gen-go=${PROTOC_GEN_GO}" \
  --go_out="${GO_OUT}" --go_opt=paths=source_relative \
  --plugin="protoc-gen-go-grpc=${PROTOC_GEN_GO_GRPC}" \
  --go-grpc_out="${GO_OUT}" --go-grpc_opt=paths=source_relative \
  "${PROTO_DIR}/semantic.proto")

echo "semantic proto stubs regenerated"
