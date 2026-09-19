-- A populated user-state boundary must never be removed by a rollback.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM native_user_state) THEN
        RAISE EXCEPTION 'native user state rollback refused: table contains data';
    END IF;
END $$;

DROP TABLE native_user_state;
