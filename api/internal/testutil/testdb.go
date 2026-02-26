// Package testutil provides shared test infrastructure for integration tests.
// It uses testcontainers-go to spin up a real PostgreSQL instance, run
// all migrations, and provide a connection pool for test services.
package testutil

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/forgecommerce/api/internal/database"
)

// TestDB holds a PostgreSQL test container and connection pool.
// It is designed to be shared across tests in a single package via
// TestMain. Each test should call Truncate() to reset state.
type TestDB struct {
	Pool      *pgxpool.Pool
	container testcontainers.Container
	connStr   string
}

// SetupTestDB starts a PostgreSQL container, runs all migrations, and
// returns a TestDB with an active connection pool.
//
// Usage in TestMain:
//
//	var testDB *testutil.TestDB
//
//	func TestMain(m *testing.M) {
//	    var code int
//	    defer func() { os.Exit(code) }()
//
//	    db, err := testutil.SetupTestDB()
//	    if err != nil { log.Fatal(err) }
//	    defer db.Close()
//	    testDB = db
//
//	    code = m.Run()
//	}
func SetupTestDB() (*TestDB, error) {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("forgecommerce_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("starting postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("getting connection string: %w", err)
	}

	// Run all migrations.
	if err := database.Migrate(connStr); err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	return &TestDB{
		Pool:      pool,
		container: container,
		connStr:   connStr,
	}, nil
}

// Close terminates the container and closes the pool.
func (tdb *TestDB) Close() {
	if tdb.Pool != nil {
		tdb.Pool.Close()
	}
	if tdb.container != nil {
		tdb.container.Terminate(context.Background())
	}
}

// Truncate removes all data from application tables while preserving
// schema. Reference tables (eu_countries, vat_categories) are preserved
// to avoid re-seeding. Call this at the start of each test for isolation.
func (tdb *TestDB) Truncate(t *testing.T) {
	t.Helper()

	// Truncate in dependency order (children first).
	tables := []string{
		"order_events",
		"order_items",
		"orders",
		"cart_items",
		"carts",
		"stock_movements",
		"variant_bom_overrides",
		"attribute_option_bom_modifiers",
		"attribute_option_bom_entries",
		"product_bom_entries",
		"product_images",
		"media_assets",
		"product_variant_options",
		"product_variants",
		"product_attribute_options",
		"product_attributes",
		"product_vat_overrides",
		"product_categories",
		"products",
		"categories",
		"raw_material_attributes",
		"raw_materials",
		"raw_material_categories",
		"coupons",
		"discounts",
		"shipping_zones",
		"shipping_configs",
		"webhook_deliveries",
		"webhook_endpoints",
		"admin_audit_log",
		"sessions",
		"admin_users",
		"customers",
		"product_variant_global_options",
		"product_global_option_selections",
		"product_global_attribute_links",
		"global_attribute_options",
		"global_attribute_metadata_fields",
		"global_attributes",
		"vies_validation_cache",
		"vat_rates",
		"store_shipping_countries",
	}

	ctx := context.Background()
	for _, table := range tables {
		_, err := tdb.Pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			// Some tables may not exist in all migration versions — skip.
			slog.Debug("truncate skipped", "table", table, "error", err.Error())
		}
	}
}

// SeedEssentials inserts the minimal reference data needed for most tests:
// EU countries and VAT categories. Call after Truncate() if your test
// needs products, orders, or other entities that reference these tables.
func (tdb *TestDB) SeedEssentials(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Seed a few EU countries (tests don't need all 27).
	_, err := tdb.Pool.Exec(ctx, `
		INSERT INTO eu_countries (country_code, name, local_vat_name, local_vat_abbreviation, is_eu_member, currency)
		VALUES
			('DE', 'Germany',  'Mehrwertsteuer',                    'MwSt.', true, 'EUR'),
			('ES', 'Spain',    'Impuesto sobre el Valor Añadido',  'IVA',   true, 'EUR'),
			('FR', 'France',   'Taxe sur la valeur ajoutée',       'TVA',   true, 'EUR')
		ON CONFLICT (country_code) DO NOTHING
	`)
	if err != nil {
		t.Fatalf("seeding EU countries: %v", err)
	}

	// Seed VAT categories.
	_, err = tdb.Pool.Exec(ctx, `
		INSERT INTO vat_categories (id, name, display_name, description, maps_to_rate_type, is_default, position)
		VALUES
			('a0000000-0000-0000-0000-000000000001', 'standard', 'Standard Rate',
			 'Default rate for most goods and services', 'standard', true, 1)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		t.Fatalf("seeding VAT categories: %v", err)
	}
}
