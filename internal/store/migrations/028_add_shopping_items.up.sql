CREATE TABLE shopping_items (
    id                UUID PRIMARY KEY,
    -- Monotonic insertion order: within one bulk insert every row shares the
    -- transaction's created_at, so seq is the tiebreaker that preserves the
    -- agent's input order on listing.
    seq               BIGINT GENERATED ALWAYS AS IDENTITY,
    name              TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 300),
    quantity_text     TEXT NULL,
    -- Soft provenance: cleared if the recipe product is later deleted; the
    -- item stays on the list (see design D3).
    recipe_product_id UUID NULL REFERENCES products(id) ON DELETE SET NULL,
    plan_date         DATE NULL,
    checked           BOOLEAN NOT NULL DEFAULT FALSE,
    checked_at        TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX shopping_items_list_idx ON shopping_items (checked, created_at, seq);
