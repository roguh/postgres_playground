package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	var (
		direction = flag.String("dir", "up", "Migration direction (up/down)")
		steps     = flag.Int("steps", 0, "Number of migrations to run")
		force     = flag.Int("force", 0, "Force to specific version")
	)
	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatal("Usage: migrate [up|down|force|version]")
	}

	action := flag.Args()[0]

	// Database connection
	dsn := getEnv("DATABASE_URL", "postgres://grug:grug_like_simple@localhost:5432/playground?sslmode=disable")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer db.Close()

	// Create migration instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatal("Failed to create driver:", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
	if err != nil {
		log.Fatal("Failed to create migrate instance:", err)
	}

	// Execute migration
	switch action {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
	case "force":
		err = m.Force(*force)
		fmt.Printf("Forced to version %d\n", *force)
	case "version":
		version, dirty, verr := m.Version()
		if verr != nil {
			log.Fatal("Failed to get version:", verr)
		}
		fmt.Printf("Version: %d, Dirty: %v\n", version, dirty)
		return
	default:
		log.Fatal("Unknown action:", action)
	}

	if err != nil && err != migrate.ErrNoChange {
		log.Fatal("Migration failed:", err)
	}

	if err == migrate.ErrNoChange {
		fmt.Println("No changes to apply")
	} else {
		fmt.Printf("Migration %s completed successfully\n", action)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
