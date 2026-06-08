ALTER TABLE products
    ADD COLUMN last_logged_quantity_g NUMERIC(10, 3);

-- One-shot backfill: populate the new column from the most recent meal entry
-- per product, so the phone immediately gets useful default quantities for
-- products that have a log history. Safe on empty databases (no rows touched).
-- For very large meal_entries tables this scan may be a few seconds; remove
-- this UPDATE and re-run the migration without it if operating against a
-- database where that's not acceptable.
UPDATE products
SET last_logged_quantity_g = me.quantity_g
FROM (
    SELECT DISTINCT ON (product_id)
        product_id,
        quantity_g
    FROM meal_entries
    WHERE product_id IS NOT NULL
    ORDER BY product_id, logged_at DESC
) AS me
WHERE products.id = me.product_id;
