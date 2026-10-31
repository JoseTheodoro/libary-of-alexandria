package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitPersistsAndEnablesFTS5AndForeignKeys(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "banco com espaço #1.sqlite")
	s, err := Init(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO fragment_fts(rowid, title, body) VALUES (1, 'Fonte', 'ação e evidência')`); err != nil {
		t.Fatalf("FTS5 insert: %v", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM fragment_fts WHERE fragment_fts MATCH ?`, "evidência").Scan(&count); err != nil || count != 1 {
		t.Fatalf("FTS5 search: count=%d, err=%v", count, err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO source_revisions(source_id, sha256) VALUES (?, ?)`, "absent", strings.Repeat("a", 64)); err == nil {
		t.Fatal("foreign key não foi aplicada")
	}
	// Force a replacement connection: DSN settings must apply to every connection.
	s.db.SetConnMaxLifetime(time.Nanosecond)
	var enabled int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
		t.Fatalf("foreign keys na reconexão: %d, %v", enabled, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Init(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fragment_fts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("dados após reabertura: %d, %v", count, err)
	}
	if err := reopened.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration duplicada: %d, %v", count, err)
	}
}

func TestInitRejectsAlteredOrNewerMigration(t *testing.T) {
	for _, test := range []struct {
		name  string
		query string
	}{
		{"checksum alterado", `UPDATE schema_migrations SET checksum='different' WHERE version=1`},
		{"versão futura", `INSERT INTO schema_migrations(version, checksum) VALUES (2, 'future')`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.sqlite")
			s, err := Init(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(test.query); err != nil {
				t.Fatal(err)
			}
			s.Close()
			if reopened, err := Init(context.Background(), path); err == nil {
				reopened.Close()
				t.Fatal("banco incompatível aceito")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("banco original foi removido: %v", err)
			}
		})
	}
}

func TestMigrationFailureRollsBackWithoutDeletingExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE sources(marker TEXT); INSERT INTO sources VALUES ('preserve')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Init(context.Background(), path); err == nil {
		s.Close()
		t.Fatal("migration conflitante aceita")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var marker string
	if err := db.QueryRow("SELECT marker FROM sources").Scan(&marker); err != nil || marker != "preserve" {
		t.Fatalf("dados originais não preservados: %q, %v", marker, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='schema_migrations'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("migration não revertida: count=%d, err=%v", count, err)
	}
}

func TestInitRejectsEmptyAndMemoryPaths(t *testing.T) {
	for _, path := range []string{"", "   ", ":memory:"} {
		if s, err := Init(context.Background(), path); err == nil {
			s.Close()
			t.Fatalf("caminho %q aceito", path)
		}
	}
}

func TestCanonicalContentSurvivesIndexDeletionAndRejectsWrongSource(t *testing.T) {
	ctx := context.Background()
	s, err := Init(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	queries := []string{
		`INSERT INTO sources(id,title,authors_json,format,permission_json) VALUES ('a','A','["Test"]','pdf','{}'),('b','B','["Test"]','pdf','{}')`,
		`INSERT INTO source_revisions(source_id,sha256) VALUES ('a','sha-a'),('b','sha-b')`,
		`INSERT INTO extraction_revisions(id,source_id,source_sha256,events_sha256,contract_version,engine_json,mode,status) VALUES ('extract-a','a','sha-a','events',1,'{}','layout','complete')`,
		`INSERT INTO chunk_profiles(id,config_json) VALUES ('profile','{}')`,
	}
	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	const original = "Ação 📚\nfunc ReadFile() {\n    return value\n}\n"
	if _, err := s.db.ExecContext(ctx, `INSERT INTO document_units(extraction_id,unit_id,ordinal,text,locator_json) VALUES(?,?,?,?,?)`, "extract-a", "page-000001", 1, original, `{"kind":"pdf_page","page":1}`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO fragments(stable_id,extraction_id,unit_id,profile_id,start_byte,end_byte) VALUES(?,?,?,?,?,?)`, "fragment-a", "extract-a", "page-000001", "profile", 0, len(original)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO active_documents(source_id,extraction_id,profile_id) VALUES ('b','extract-a','profile')`); err == nil {
		t.Fatal("extração vinculada à fonte errada foi aceita")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO active_documents(source_id,extraction_id,profile_id) VALUES ('a','extract-a','profile')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO fragment_fts(rowid,title,body) SELECT id,'A',? FROM fragments WHERE stable_id='fragment-a'`, original); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM fragment_fts`); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := s.db.QueryRowContext(ctx, `SELECT text FROM document_units WHERE extraction_id=? AND unit_id=?`, "extract-a", "page-000001").Scan(&got); err != nil || got != original {
		t.Fatalf("conteúdo canônico alterado: %q, %v", got, err)
	}
}
