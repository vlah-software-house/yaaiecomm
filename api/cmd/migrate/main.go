package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/forgecommerce/api/internal/database"
)

func main() {
	direction := flag.String("direction", "up", "Migration direction: up or down")
	dbURL := flag.String("db", "", "Database URL")
	steps := flag.Int("steps", 0, "Number of steps (only for down)")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}
	if *dbURL == "" {
		log.Fatal("database URL is required: use -db flag or DATABASE_URL env var")
	}

	switch *direction {
	case "up":
		if err := database.Migrate(*dbURL); err != nil {
			log.Fatalf("migration up failed: %v", err)
		}
		fmt.Println("migrations applied successfully")
	case "down":
		if *steps <= 0 {
			*steps = 1
		}
		for i := 0; i < *steps; i++ {
			if err := database.MigrateDown(*dbURL); err != nil {
				log.Fatalf("migration down (step %d) failed: %v", i+1, err)
			}
		}
		fmt.Printf("rolled back %d migration(s)\n", *steps)
	default:
		log.Fatalf("unknown direction: %s (use 'up' or 'down')", *direction)
	}
}
