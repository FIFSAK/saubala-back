package store

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "modernc.org/sqlite"
)

type SQL struct {
	Connection *sql.DB
}

func NewSQL(dsn string) (*SQL, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("store: empty data source name")
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Printf("store: connection failed dsn=%s err=%v", dsn, err)
		return nil, fmt.Errorf("store: connect failed: err=%w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("store: set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("store: enable foreign keys: %w", err)
	}

	if err = db.Ping(); err != nil {
		log.Printf("store: ping failed dsn=%s err=%v", dsn, err)
		return nil, fmt.Errorf("store: ping failed: err=%w", err)
	}

	log.Printf("store: connected dsn=%s", dsn)

	return &SQL{Connection: db}, nil
}
