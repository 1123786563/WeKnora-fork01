DROP TRIGGER IF EXISTS trg_craft_capture_block_new_run ON agent_runs;
DROP FUNCTION IF EXISTS craft_capture_block_new_run();
DROP TRIGGER IF EXISTS trg_craft_capture_terminal_enqueue ON agent_runs;
DROP FUNCTION IF EXISTS craft_capture_terminal_enqueue();
DROP TRIGGER IF EXISTS trg_craft_capture_file_immutable ON craft_run_capture_files;
DROP TRIGGER IF EXISTS trg_craft_capture_identity_immutable ON craft_run_captures;
DROP FUNCTION IF EXISTS craft_capture_reject_mutation();
DROP TABLE IF EXISTS craft_run_capture_files;
DROP TABLE IF EXISTS craft_run_captures;
