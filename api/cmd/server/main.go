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
	"github.com/forgecommerce/api/internal/middleware"
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

	// Initialize admin handlers
	adminHandler := adminhandlers.NewHandler(authService, logger)

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

	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      apiMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
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

	if err := adminServer.Shutdown(ctx); err != nil {
		slog.Error("admin server shutdown error", "error", err)
	}
	if err := apiServer.Shutdown(ctx); err != nil {
		slog.Error("api server shutdown error", "error", err)
	}

	slog.Info("servers stopped")
}
