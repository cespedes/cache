CREATE TABLE locations (
	    id          SERIAL PRIMARY KEY,
	    name        TEXT NOT NULL,
	    parent_id   INTEGER REFERENCES locations(id) ON DELETE RESTRICT,
	    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE items (
	    id          SERIAL PRIMARY KEY,
	    name        TEXT NOT NULL,
	    location_id INTEGER NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
	    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER locations_updated_at
    BEFORE UPDATE ON locations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER items_updated_at
    BEFORE UPDATE ON items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
