package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/forgecommerce/api/internal/config"
	"github.com/forgecommerce/api/internal/database"
	adminhandlers "github.com/forgecommerce/api/internal/handlers/admin"
	apihandlers "github.com/forgecommerce/api/internal/handlers/api"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/attribute"
	"github.com/forgecommerce/api/internal/services/bom"
	"github.com/forgecommerce/api/internal/services/cart"
	"github.com/forgecommerce/api/internal/services/category"
	"github.com/forgecommerce/api/internal/services/discount"
	"github.com/forgecommerce/api/internal/services/order"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/internal/services/rawmaterial"
	"github.com/forgecommerce/api/internal/services/shipping"
	"github.com/forgecommerce/api/internal/services/variant"
	"github.com/forgecommerce/api/internal/vat"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadDev()

	// Connect to database
	pool, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("database connected")

	// Run migrations
	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	slog.Info("migrations complete")

	// Initialize auth service
	sessionMgr := auth.NewSessionManager(pool, 8*time.Hour)
	authService := auth.NewService(pool, sessionMgr, logger, cfg.TOTPIssuer)

	// Initialize VAT services
	vatCache := vat.NewRateCache()
	vatSyncer := vat.NewRateSyncer(pool, cfg.VAT, logger, vatCache)
	vatScheduler := vat.NewScheduler(vatSyncer, logger)

	// Initialize services
	productSvc := product.NewService(pool, logger)
	categorySvc := category.NewService(pool, logger)
	rawMaterialSvc := rawmaterial.NewService(pool, logger)
	attributeSvc := attribute.NewService(pool, logger)
	variantSvc := variant.NewService(pool, logger)
	bomSvc := bom.NewService(pool, logger)
	orderSvc := order.NewService(pool, logger)
	discountSvc := discount.NewService(pool, logger)
	shippingSvc := shipping.NewService(pool, logger)
	cartSvc := cart.NewService(pool, logger)

	// Initialize public API handlers
	publicHandler := apihandlers.NewPublicHandler(productSvc, categorySvc, variantSvc, pool, logger)
	cartHandler := apihandlers.NewCartHandler(cartSvc, logger)

	// Initialize admin handlers
	adminHandler := adminhandlers.NewHandler(authService, logger)
	productHandler := adminhandlers.NewProductHandler(productSvc, categorySvc, logger)
	categoryHandler := adminhandlers.NewCategoryHandler(categorySvc, logger)
	rawMaterialHandler := adminhandlers.NewRawMaterialHandler(rawMaterialSvc, logger)
	settingsHandler := adminhandlers.NewSettingsHandler(pool, vatSyncer, logger)
	productVATHandler := adminhandlers.NewProductVATHandler(productSvc, pool, logger)
	attributeHandler := adminhandlers.NewAttributeHandler(attributeSvc, productSvc, logger)
	variantHandler := adminhandlers.NewVariantHandler(variantSvc, attributeSvc, productSvc, logger)
	bomHandler := adminhandlers.NewBOMHandler(bomSvc, productSvc, rawMaterialSvc, variantSvc, logger)
	orderHandler := adminhandlers.NewOrderHandler(orderSvc, logger)
	discountHandler := adminhandlers.NewDiscountHandler(discountSvc, logger)
	shippingHandler := adminhandlers.NewShippingHandler(shippingSvc, logger)

	// Admin server (HTMX + templ)
	adminMux := http.NewServeMux()

	// Health check
	adminMux.HandleFunc("GET /admin/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	// Static files
	adminMux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public admin routes (login, 2FA)
	adminHandler.RegisterRoutes(adminMux)

	// Protected admin routes — wrap in auth middleware
	protectedMux := http.NewServeMux()
	adminHandler.RegisterProtectedRoutes(protectedMux)
	productHandler.RegisterRoutes(protectedMux)
	categoryHandler.RegisterRoutes(protectedMux)
	rawMaterialHandler.RegisterRoutes(protectedMux)
	settingsHandler.RegisterRoutes(protectedMux)
	productVATHandler.RegisterRoutes(protectedMux)
	attributeHandler.RegisterRoutes(protectedMux)
	variantHandler.RegisterRoutes(protectedMux)
	bomHandler.RegisterRoutes(protectedMux)
	orderHandler.RegisterRoutes(protectedMux)
	discountHandler.RegisterRoutes(protectedMux)
	shippingHandler.RegisterRoutes(protectedMux)
	adminMux.Handle("/admin/", middleware.RequireAuth(authService)(protectedMux))

	// Root redirect
	adminMux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	})

	// Apply global middleware stack
	var adminChain http.Handler = adminMux
	adminChain = middleware.CSRF(adminChain)
	adminChain = middleware.SecurityHeaders(adminChain)
	adminChain = middleware.Recover(logger)(adminChain)
	adminChain = middleware.RequestLogger(logger)(adminChain)

	adminServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AdminPort),
		Handler:      adminChain,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// API server (JSON REST)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	// Register public API routes
	publicHandler.RegisterRoutes(apiMux)
	cartHandler.RegisterRoutes(apiMux)

	// Apply API middleware stack (CORS for storefront, logging, recovery)
	var apiChain http.Handler = apiMux
	apiChain = middleware.CORS(cfg.BaseURL)(apiChain)
	apiChain = middleware.Recover(logger)(apiChain)
	apiChain = middleware.RequestLogger(logger)(apiChain)

	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      apiChain,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start VAT rate sync scheduler
	if cfg.VAT.SyncEnabled {
		vatScheduler.Start(context.Background())
	}

	// Start servers
	errCh := make(chan error, 2)

	go func() {
		slog.Info("admin server starting", "port", cfg.AdminPort)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("admin server: %w", err)
		}
	}()

	go func() {
		slog.Info("API server starting", "port", cfg.Port)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig)
	case err := <-errCh:
		slog.Error("server error", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Stop VAT scheduler
	vatScheduler.Stop()

	if err := adminServer.Shutdown(ctx); err != nil {
		slog.Error("admin server shutdown error", "error", err)
	}
	if err := apiServer.Shutdown(ctx); err != nil {
		slog.Error("api server shutdown error", "error", err)
	}

	slog.Info("servers stopped")
}
