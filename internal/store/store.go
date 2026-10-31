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

//go:embed migrations/002_adapter_digest.sql
var adapterDigestMigration string

var migrations = []string{initialMigration, adapterDigestMigration}

const SchemaVersion = 2

type Store struct{ db *sql.DB }

// OpenReadOnly refuses absent or incompatible databases and never runs migrations.
func OpenReadOnly(ctx context.Context, path string) (*Store, error) {
	return openExisting(ctx, path, "ro")
}

// OpenExisting permits explicit maintenance without creating or migrating a database.
func OpenExisting(ctx context.Context, path string) (*Store, error) {
	return openExisting(ctx, path, "rw")
}

func openExisting(ctx context.Context, path, mode string) (*Store, error) {
	if strings.TrimSpace(path) == "" || path == ":memory:" {
		return nil, fmt.Errorf("informe um caminho de banco local em disco")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("mode", mode)
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: abs, RawQuery: query.Encode()}).String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	rows, err := db.QueryContext(ctx, `SELECT version,checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("banco ausente ou schema não inicializado: %w", err)
	}
	count := 0
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			db.Close()
			return nil, err
		}
		if version != count+1 || version > len(migrations) {
			rows.Close()
			db.Close()
			return nil, fmt.Errorf("migration desconhecida ou ausente")
		}
		hash := sha256.Sum256([]byte(migrations[version-1]))
		if checksum != hex.EncodeToString(hash[:]) {
			rows.Close()
			db.Close()
			return nil, fmt.Errorf("migration alterada")
		}
		count++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		db.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		db.Close()
		return nil, err
	}
	if count != len(migrations) {
		db.Close()
		return nil, fmt.Errorf("schema incompatível; execute store init explicitamente")
	}
	return &Store{db: db}, nil
}

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
	applied := 0
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return err
		}
		if version != applied+1 || version > len(migrations) {
			rows.Close()
			return fmt.Errorf("migration desconhecida ou alterada: versão %d", version)
		}
		digest := sha256.Sum256([]byte(migrations[version-1]))
		if checksum != hex.EncodeToString(digest[:]) {
			rows.Close()
			return fmt.Errorf("migration desconhecida ou alterada: versão %d", version)
		}
		applied++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for index := applied; index < len(migrations); index++ {
		if _, err := tx.ExecContext(ctx, migrations[index]); err != nil {
			return fmt.Errorf("aplicar migration %d (FTS5 necessário): %w", index+1, err)
		}
		digest := sha256.Sum256([]byte(migrations[index]))
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, checksum) VALUES(?, ?)", index+1, hex.EncodeToString(digest[:])); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("concluir migration: %w", err)
	}
	return nil
}
