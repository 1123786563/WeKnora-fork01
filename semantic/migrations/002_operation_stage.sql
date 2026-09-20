ALTER TABLE {{schema}}.operations ADD COLUMN IF NOT EXISTS stage TEXT;
UPDATE {{schema}}.operations SET stage = phase WHERE stage IS NULL;
ALTER TABLE {{schema}}.operations ALTER COLUMN stage SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'operations_lease_token_uint64_check'
          AND conrelid = '{{schema_regclass}}'::regclass
    ) THEN
        ALTER TABLE {{schema}}.operations
            ADD CONSTRAINT operations_lease_token_uint64_check
            CHECK (lease_token >= 0 AND lease_token <= 18446744073709551615);
    END IF;
END $$;
