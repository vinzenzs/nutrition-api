-- Extend allowed product sources to include 'recipe'.
ALTER TABLE products
    DROP CONSTRAINT products_source_check;

ALTER TABLE products
    ADD CONSTRAINT products_source_check
    CHECK (source IN ('off', 'manual', 'recipe'));

-- Track when a recipe's nutriments were last computed from its components.
ALTER TABLE products
    ADD COLUMN nutriment_computed_at TIMESTAMPTZ;

CREATE TABLE product_components (
    id                   UUID PRIMARY KEY,
    product_id           UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    component_product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    quantity_g           NUMERIC(10, 3) NOT NULL CHECK (quantity_g > 0),
    position             INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX product_components_product_id_idx ON product_components (product_id);
