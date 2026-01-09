package config

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations executes all SQL files in /migrations folder in order
func RunMigrations(db *pgxpool.Pool) {
	ctx := context.Background()

	migrationsPath := "./migrations" // adjust relative path if needed

	err := filepath.WalkDir(migrationsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}

		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading migration %s failed: %w", path, err)
		}

		_, err = db.Exec(ctx, string(sqlBytes))
		if err != nil {
			return fmt.Errorf("executing migration %s failed: %w", path, err)
		}

		log.Println("Migration executed:", d.Name())
		return nil
	})

	if err != nil {
		log.Fatal("Migrations failed:", err)
	}

	log.Println("All migrations executed successfully!")
}
