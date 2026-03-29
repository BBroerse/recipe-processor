package postgres

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/bbroerse/recipe-processor/migrations"
)

// Migrate runs all pending database migrations using embedded SQL files.
// It logs the current version before and after applying migrations so
// operators can see exactly which versions were applied on each startup.
func Migrate(dsn string) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			slog.Error("failed to close migration source", "error", srcErr)
		}
		if dbErr != nil {
			slog.Error("failed to close migration database", "error", dbErr)
		}
	}()

	beforeVersion, dirty, verErr := m.Version()
	if dirty {
		slog.Warn("database migration state is dirty", "version", beforeVersion)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("no pending migrations", "current_version", beforeVersion)
			return nil
		}
		return fmt.Errorf("running migrations: %w", err)
	}

	afterVersion, _, _ := m.Version()

	if verErr != nil {
		// First-ever migration run (no previous version existed).
		slog.Info("migrations applied", "from_version", "none", "to_version", afterVersion)
	} else {
		slog.Info("migrations applied", "from_version", beforeVersion, "to_version", afterVersion)
	}

	return nil
}
