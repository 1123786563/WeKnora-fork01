from __future__ import annotations


def test_contract_exposes_only_versioned_semantica_service() -> None:
    from semantic_service.proto import semantic_pb2, semantic_pb2_grpc

    assert semantic_pb2.DESCRIPTOR.package == "weknora.semantic.v1"
    assert set(semantic_pb2.DESCRIPTOR.services_by_name["SemanticService"].methods_by_name) == {
        "GetCapabilities",
        "ApplyDocumentRevision",
        "DeleteDocument",
        "GetOperation",
        "CancelOperation",
        "Search",
        "Reason",
    }
    assert semantic_pb2_grpc.SemanticServiceStub
