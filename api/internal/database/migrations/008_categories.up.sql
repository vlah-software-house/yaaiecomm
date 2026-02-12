-- 008_categories.up.sql
-- Categories for organizing products, with self-referencing hierarchy
-- and a junction table linking products to categories (many-to-many)

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    position INTEGER NOT NULL DEFAULT 0,
    image_url TEXT,
    seo_title TEXT,
    seo_description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_categories_parent_id ON categories(parent_id);
CREATE INDEX idx_categories_slug ON categories(slug);
CREATE INDEX idx_categories_is_active ON categories(is_active);

-- Junction table: product <-> category (many-to-many)
-- Note: FK on product_id -> products(id) will be added in migration 009
-- once the products table exists.
CREATE TABLE product_categories (
    product_id UUID NOT NULL,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, category_id)
);

CREATE INDEX idx_product_categories_category_id ON product_categories(category_id);
