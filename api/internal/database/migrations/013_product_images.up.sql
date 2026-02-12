-- 013_product_images.up.sql
-- Product images with optional variant/option association
-- Priority for display: variant-specific > option-specific > product-level
-- Also: general media asset registry for file uploads

CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id UUID REFERENCES product_variants(id) ON DELETE SET NULL,
    option_id UUID REFERENCES product_attribute_options(id) ON DELETE SET NULL,
    url TEXT NOT NULL,
    alt_text TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_product_images_product_id ON product_images(product_id);
CREATE INDEX idx_product_images_variant_id ON product_images(variant_id);
CREATE INDEX idx_product_images_option_id ON product_images(option_id);

-- General media asset registry (uploaded files)
CREATE TABLE media_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    url TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
