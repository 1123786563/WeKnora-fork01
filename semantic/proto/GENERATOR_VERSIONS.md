# C01 protobuf generator versions

- Python: CPython 3.12.12
- grpcio-tools: 1.80.0 (embedded protoc 6.31.1)
- grpcio: 1.80.0
- protobuf: 6.33.6
- protoc-gen-go: 1.36.11
- protoc-gen-go-grpc: 1.5.1

Generate from repository root with `semantic/scripts/generate_proto.sh`. The
script generates Go and Python bindings from `semantic.proto`; do not use the
DocReader generator or manually edit generated files.
