// Package store initializes the local canonical SQLite database explicitly.
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_init.sql
var initialMigration string

type Store struct{ db *sql.DB }

// Init creates or migrates the database as an explicit write operation.
// The caller owns Close; no parent directories are created implicitly.
func Init(ctx context.Context, path string) (*Store, error) {
	if strings.TrimSpace(path) == "" || path == ":memory:" {
		return nil, fmt.Errorf("informe um caminho de banco local em disco")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	dsn := (&url.URL{Scheme: "file", Path: abs, RawQuery: query.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SQLiteVersion reports the engine actually loaded by the driver.
func (s *Store) SQLiteVersion(ctx context.Context) (string, error) {
	var version string
	err := s.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	return version, err
}

func (s *Store) initialize(ctx context.Context) error {
	var enabled int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
		return fmt.Errorf("foreign keys não habilitadas: %v", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, checksum TEXT NOT NULL
	)`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(initialMigration))
	expected := hex.EncodeToString(digest[:])
	applied := false
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return err
		}
		if version != 1 || checksum != expected {
			rows.Close()
			return fmt.Errorf("migration desconhecida ou alterada: versão %d", version)
		}
		applied = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if !applied {
		if _, err := tx.ExecContext(ctx, initialMigration); err != nil {
			return fmt.Errorf("aplicar migration inicial (FTS5 necessário): %w", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, checksum) VALUES(?, ?)", 1, expected); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("concluir migration: %w", err)
	}
	return nil
}
