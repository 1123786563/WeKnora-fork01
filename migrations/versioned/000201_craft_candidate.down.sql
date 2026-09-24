DROP TRIGGER IF EXISTS trg_craft_candidate_file_no_update ON craft_candidate_files;
DROP TRIGGER IF EXISTS trg_craft_candidate_no_update ON craft_candidates;
DROP FUNCTION IF EXISTS craft_reject_candidate_mutation();
DROP TABLE IF EXISTS craft_candidate_files;
DROP TABLE IF EXISTS craft_candidates;
