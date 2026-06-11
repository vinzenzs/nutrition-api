-- Recipe products imported from Cookidoo (and via the Chrome extension) carry an
-- ordered list of free-text ingredient strings extracted from the page's
-- Schema.org Recipe JSON-LD (e.g. "100 g Staudensellerie"). Stored verbatim as a
-- JSON array; no ingredient->product linking and no quantity parsing — the
-- shopping-list agent does that synthesis. Null for every non-recipe product and
-- for recipes whose source provided no ingredient list.

ALTER TABLE products ADD COLUMN ingredients JSONB NULL;
