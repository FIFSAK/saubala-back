package store

import (
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn string) error {
	migrationsPath := "file://migrations/sqlite"

	log.Printf("migrate: start driver=sqlite path=%s", migrationsPath)

	dbURL := fmt.Sprintf("sqlite://%s", dsn)

	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("migrate: new: %w", err)
	}

	defer func() {
		serr, derr := m.Close()
		if derr != nil || serr != nil {
			log.Printf("migrate: close error: serr=%v, derr=%v", serr, derr)
		}
	}()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Printf("migrate: no-change driver=sqlite")
			return nil
		}
		return fmt.Errorf("migrate: up: %w", err)
	}

	log.Printf("migrate: applied driver=sqlite")
	return nil
}
