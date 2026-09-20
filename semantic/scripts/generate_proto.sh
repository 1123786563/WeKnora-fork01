#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
python_bin=${PYTHON_BIN:-"$root/semantic/.venv/bin/python"}
go_bin=${PROTOC_GO_BIN:-"${GOPATH:-$HOME/go}/bin"}

test -x "$python_bin"
test -x "$go_bin/protoc-gen-go"
test -x "$go_bin/protoc-gen-go-grpc"

PATH="$go_bin:$PATH" "$python_bin" -m grpc_tools.protoc -I "$root/semantic/proto" \
  --python_out="$root/semantic/semantic_service/proto" \
  --pyi_out="$root/semantic/semantic_service/proto" \
  --grpc_python_out="$root/semantic/semantic_service/proto" \
  --go_out="$root/semantic/proto" --go_opt=paths=source_relative \
  --go-grpc_out="$root/semantic/proto" --go-grpc_opt=paths=source_relative \
  "$root/semantic/proto/semantic.proto"

python_file="$root/semantic/semantic_service/proto/semantic_pb2_grpc.py"
sed -i.bak 's/^import semantic_pb2 as semantic__pb2/from semantic_service.proto import semantic_pb2 as semantic__pb2/' "$python_file"
rm "$python_file.bak"
