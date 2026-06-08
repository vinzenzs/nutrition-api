CREATE INDEX products_name_lower_idx          ON products (lower(name));
CREATE INDEX products_brand_lower_idx         ON products (lower(brand));
CREATE INDEX products_last_logged_at_desc_idx ON products (last_logged_at DESC NULLS LAST);
CREATE INDEX meal_entries_logged_at_idx       ON meal_entries (logged_at);
CREATE INDEX meal_entries_product_id_idx      ON meal_entries (product_id);
