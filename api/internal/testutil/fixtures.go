package testutil

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// FixtureProduct creates a minimal active product and returns it.
func (tdb *TestDB) FixtureProduct(t *testing.T, name, slug string) db.Product {
	t.Helper()
	q := db.New(tdb.Pool)
	ctx := context.Background()

	now := time.Now().UTC()
	product, err := q.CreateProduct(ctx, db.CreateProductParams{
		ID:                      uuid.New(),
		Name:                    name,
		Slug:                    slug,
		Status:                  "active",
		BasePrice:               pgtype.Numeric{Int: big.NewInt(2500), Exp: -2, Valid: true}, // 25.00
		ShippingExtraFeePerUnit: pgtype.Numeric{Int: big.NewInt(0), Exp: -2, Valid: true},
		HasVariants:             false,
		Metadata:                json.RawMessage(`{}`),
		CreatedAt:               now,
	})
	if err != nil {
		t.Fatalf("creating fixture product %q: %v", name, err)
	}
	return product
}

// FixtureVariant creates a minimal active variant for a product and returns it.
func (tdb *TestDB) FixtureVariant(t *testing.T, productID uuid.UUID, sku string, stock int32) db.ProductVariant {
	t.Helper()
	q := db.New(tdb.Pool)
	ctx := context.Background()

	variant, err := q.CreateProductVariant(ctx, db.CreateProductVariantParams{
		ID:            uuid.New(),
		ProductID:     productID,
		Sku:           sku,
		Price:         pgtype.Numeric{Int: big.NewInt(2500), Exp: -2, Valid: true},
		StockQuantity: stock,
		IsActive:      true,
		Position:      1,
	})
	if err != nil {
		t.Fatalf("creating fixture variant %q: %v", sku, err)
	}
	return variant
}

// FixtureRawMaterial creates a minimal active raw material and returns it.
func (tdb *TestDB) FixtureRawMaterial(t *testing.T, name, sku string) db.RawMaterial {
	t.Helper()
	q := db.New(tdb.Pool)
	ctx := context.Background()

	now := time.Now().UTC()
	material, err := q.CreateRawMaterial(ctx, db.CreateRawMaterialParams{
		ID:                uuid.New(),
		Name:              name,
		Sku:               sku,
		UnitOfMeasure:     "unit",
		CostPerUnit:       pgtype.Numeric{Int: big.NewInt(500), Exp: -2, Valid: true}, // 5.00
		StockQuantity:     pgtype.Numeric{Int: big.NewInt(100), Exp: 0, Valid: true},
		LowStockThreshold: pgtype.Numeric{Int: big.NewInt(10), Exp: 0, Valid: true},
		Metadata:          json.RawMessage(`{}`),
		IsActive:          true,
		CreatedAt:         now,
	})
	if err != nil {
		t.Fatalf("creating fixture raw material %q: %v", name, err)
	}
	return material
}

// FixtureAttributeOption creates a product attribute with a single option and returns the option.
// This is a convenience for BOM tests that need an attribute option to link entries/modifiers to.
func (tdb *TestDB) FixtureAttributeOption(t *testing.T, productID uuid.UUID, attrName, optionValue string) db.ProductAttributeOption {
	t.Helper()
	q := db.New(tdb.Pool)
	ctx := context.Background()

	attr, err := q.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:            uuid.New(),
		ProductID:     productID,
		Name:          attrName,
		DisplayName:   attrName,
		AttributeType: "select",
		Position:      1,
	})
	if err != nil {
		t.Fatalf("creating fixture attribute %q: %v", attrName, err)
	}

	option, err := q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
		ID:           uuid.New(),
		AttributeID:  attr.ID,
		Value:        optionValue,
		DisplayValue: optionValue,
		Position:     1,
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("creating fixture option %q: %v", optionValue, err)
	}
	return option
}

// FixtureCategory creates a minimal active category and returns it.
func (tdb *TestDB) FixtureCategory(t *testing.T, name, slug string) db.Category {
	t.Helper()
	q := db.New(tdb.Pool)
	ctx := context.Background()

	now := time.Now().UTC()
	cat, err := q.CreateCategory(ctx, db.CreateCategoryParams{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		IsActive:  true,
		Position:  1,
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("creating fixture category %q: %v", name, err)
	}
	return cat
}

// FixtureShippingCountry enables a shipping country (must be seeded in eu_countries first).
func (tdb *TestDB) FixtureShippingCountry(t *testing.T, countryCode string) {
	t.Helper()
	ctx := context.Background()

	_, err := tdb.Pool.Exec(ctx,
		`INSERT INTO store_shipping_countries (country_code, is_enabled, position)
		 VALUES ($1, true, 0)
		 ON CONFLICT (country_code) DO UPDATE SET is_enabled = true`,
		countryCode,
	)
	if err != nil {
		t.Fatalf("enabling fixture shipping country %q: %v", countryCode, err)
	}
}

// FixtureCustomer creates a minimal customer and returns it.
func (tdb *TestDB) FixtureCustomer(t *testing.T, email string) db.Customer {
	t.Helper()
	ctx := context.Background()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := tdb.Pool.Exec(ctx,
		`INSERT INTO customers (id, email, password_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)`,
		id, email, "$2a$12$placeholder_hash_for_testing", now,
	)
	if err != nil {
		t.Fatalf("creating fixture customer %q: %v", email, err)
	}

	return db.Customer{
		ID:        id,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
