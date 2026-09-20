#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
go_bin=${PROTOC_GO_BIN:-"${GOPATH:-$HOME/go}/bin"}
mode=${1:-generate}

test -x "$go_bin/protoc-gen-go"
test -x "$go_bin/protoc-gen-go-grpc"
test "$(uv run --locked --project "$root/semantic" python -c 'from importlib.metadata import version; print(version("grpcio-tools"))')" = "1.80.0"
test "$(uv run --locked --project "$root/semantic" python -c 'from importlib.metadata import version; print(version("grpcio"))')" = "1.80.0"
test "$(uv run --locked --project "$root/semantic" python -c 'from importlib.metadata import version; print(version("protobuf"))')" = "6.33.6"
test "$(uv run --locked --project "$root/semantic" python -m grpc_tools.protoc --version)" = "libprotoc 31.1"
test "$("$go_bin/protoc-gen-go" --version)" = "protoc-gen-go v1.36.11"
test "$("$go_bin/protoc-gen-go-grpc" --version)" = "protoc-gen-go-grpc 1.5.1"

PATH="$go_bin:$PATH" uv run --locked --project "$root/semantic" python -m grpc_tools.protoc -I "$root/semantic/proto" \
  --python_out="$root/semantic/semantic_service/proto" \
  --pyi_out="$root/semantic/semantic_service/proto" \
  --grpc_python_out="$root/semantic/semantic_service/proto" \
  --go_out="$root/semantic/proto" --go_opt=paths=source_relative \
  --go-grpc_out="$root/semantic/proto" --go-grpc_opt=paths=source_relative \
  "$root/semantic/proto/semantic.proto"

python_file="$root/semantic/semantic_service/proto/semantic_pb2_grpc.py"
sed -i.bak 's/^import semantic_pb2 as semantic__pb2/from semantic_service.proto import semantic_pb2 as semantic__pb2/' "$python_file"
rm "$python_file.bak"

case "$mode" in
  generate) ;;
  --check) git -C "$root" diff --exit-code -- semantic/proto/semantic.pb.go semantic/proto/semantic_grpc.pb.go semantic/semantic_service/proto/semantic_pb2.py semantic/semantic_service/proto/semantic_pb2.pyi semantic/semantic_service/proto/semantic_pb2_grpc.py ;;
  *) echo "usage: $0 [--check]" >&2; exit 2 ;;
esac
