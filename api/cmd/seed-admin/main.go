package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/forgecommerce/api/internal/config"
	"github.com/forgecommerce/api/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.LoadDev()

	pool, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	sessionMgr := auth.NewSessionManager(pool, 0)
	authService := auth.NewService(pool, sessionMgr, logger, cfg.TOTPIssuer)

	email := "admin@forgecommerce.local"
	password := "admin123"
	name := "Admin"

	if len(os.Args) > 1 {
		email = os.Args[1]
	}
	if len(os.Args) > 2 {
		password = os.Args[2]
	}

	// Check if user already exists
	_, err = authService.GetUserByEmail(context.Background(), email)
	if err == nil {
		fmt.Printf("Admin user %s already exists\n", email)
		os.Exit(0)
	}

	user, err := authService.CreateUser(context.Background(), email, name, password, "super_admin", []string{"*"})
	if err != nil {
		slog.Error("failed to create admin user", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Admin user created:\n  ID:    %s\n  Email: %s\n  Name:  %s\n  Role:  %s\n\nLogin at http://localhost:8081/admin/login\nYou will be prompted to set up 2FA on first login.\n", user.ID, user.Email, user.Name, user.Role)
}
