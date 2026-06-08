-- Record the upstream source for products that came from somewhere other than
-- direct manual entry, the OFF lookup, or an internal recipe composition.
-- Today's first user is the Chrome extension that imports Cookidoo recipes;
-- the column is generic enough to hold any URL.

ALTER TABLE products ADD COLUMN external_url TEXT NULL;
