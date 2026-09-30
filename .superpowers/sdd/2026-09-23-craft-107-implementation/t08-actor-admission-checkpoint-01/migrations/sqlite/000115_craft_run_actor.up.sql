-- Store the authenticated initiating user separately from the Task storage owner.
-- Historical actor identity remains unknown; do not backfill from owner_id.
ALTER TABLE agent_runs
    ADD COLUMN actor_user_id VARCHAR(512) NULL;
