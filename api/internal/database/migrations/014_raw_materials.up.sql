-- 014_raw_materials.up.sql
-- Raw materials inventory for manufacturers
-- Includes categories, material attributes, and stock tracking

CREATE TABLE raw_material_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    parent_id UUID REFERENCES raw_material_categories(id) ON DELETE SET NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE raw_materials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    sku TEXT UNIQUE NOT NULL,
    description TEXT,
    category_id UUID REFERENCES raw_material_categories(id) ON DELETE SET NULL,
    unit_of_measure TEXT NOT NULL DEFAULT 'unit' CHECK (unit_of_measure IN ('unit', 'kg', 'g', 'm', 'm2', 'm3', 'l', 'ml')),
    cost_per_unit NUMERIC(12,4) NOT NULL DEFAULT 0,
    stock_quantity NUMERIC(12,4) NOT NULL DEFAULT 0,
    low_stock_threshold NUMERIC(12,4) NOT NULL DEFAULT 0,
    supplier_name TEXT,
    supplier_sku TEXT,
    lead_time_days INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_raw_materials_sku ON raw_materials(sku);
CREATE INDEX idx_raw_materials_category_id ON raw_materials(category_id);
CREATE INDEX idx_raw_materials_is_active ON raw_materials(is_active);

CREATE TABLE raw_material_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_material_id UUID NOT NULL REFERENCES raw_materials(id) ON DELETE CASCADE,
    attribute_name TEXT NOT NULL,
    attribute_value TEXT NOT NULL,
    UNIQUE (raw_material_id, attribute_name)
);

CREATE INDEX idx_raw_material_attributes_raw_material_id ON raw_material_attributes(raw_material_id);
