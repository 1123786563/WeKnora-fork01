CREATE OR REPLACE FUNCTION craft_web_build_receipt_no_mutation_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'craft web build receipt is immutable';
END;
$$;
