DROP TRIGGER IF EXISTS craft_docker_output_identity_immutable ON craft_docker_output_operations;
DROP FUNCTION IF EXISTS craft_docker_output_identity_immutable_fn();
DROP TRIGGER IF EXISTS craft_docker_output_scope_validate ON craft_docker_output_operations;
DROP FUNCTION IF EXISTS craft_docker_output_scope_validate_fn();
DROP TABLE IF EXISTS craft_docker_output_chunks;
DROP TABLE IF EXISTS craft_docker_output_operations;
