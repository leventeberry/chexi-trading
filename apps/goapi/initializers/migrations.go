package initializers

import (
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"goapi/config"
	"goapi/logger"
)

// RunVersionedMigrations applies SQL files from migrationsDir (default "migrations") using golang-migrate.
func RunVersionedMigrations(cfg *config.Config, migrationsDir string) error {
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	abs, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("migrations path: %w", err)
	}
	url := config.PostgresURL(cfg)
	src := fmt.Sprintf("file://%s", filepath.ToSlash(abs))
	m, err := migrate.New(src, url)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	logger.Log.Info().Str("dir", abs).Msg("Versioned SQL migrations applied")
	return nil
}
