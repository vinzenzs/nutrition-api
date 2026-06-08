DROP INDEX IF EXISTS product_components_product_id_idx;
DROP TABLE IF EXISTS product_components;

ALTER TABLE products
    DROP COLUMN IF EXISTS nutriment_computed_at;

ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_source_check;

ALTER TABLE products
    ADD CONSTRAINT products_source_check
    CHECK (source IN ('off', 'manual'));
