package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkPreservesCanonicalTextAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	source, artifact := syntheticArtifact(t)
	s, err := Init(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	imported, err := s.Import(ctx, source, artifact, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("ação 📚\n    code()\n", 400)
	if _, err := s.db.ExecContext(ctx, `UPDATE document_units SET text=? WHERE extraction_id=?`, text, imported.ExtractionID); err != nil {
		t.Fatal(err)
	}
	first, err := s.Chunk(ctx, imported.ExtractionID)
	if err != nil || first.Fragments != 1 || first.Oversized != 1 {
		t.Fatalf("%+v %v", first, err)
	}
	var id, actual string
	var start, end int
	if err := s.db.QueryRowContext(ctx, `SELECT stable_id,start_byte,end_byte FROM fragments`).Scan(&id, &start, &end); err != nil {
		t.Fatal(err)
	}
	if start != 0 || end != len(text) || text[start:end] != text {
		t.Fatal("fragmento não preservou os bytes originais")
	}
	second, err := s.Chunk(ctx, imported.ExtractionID)
	if err != nil || second != first {
		t.Fatalf("repetição: %+v %v", second, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT stable_id FROM fragments`).Scan(&actual); err != nil || actual != id {
		t.Fatalf("identidade instável: %s %v", actual, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE document_units SET text=?`, strings.Replace(text, "code", "nope", 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Chunk(ctx, imported.ExtractionID); err == nil {
		t.Fatal("texto alterado aceito silenciosamente")
	}
}

func TestChunkFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	source, artifact := syntheticArtifact(t)
	s, err := Init(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	imported, err := s.Import(ctx, source, artifact, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_fragment BEFORE INSERT ON fragments BEGIN SELECT RAISE(ABORT,'failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Chunk(ctx, imported.ExtractionID); err == nil {
		t.Fatal("falha ignorada")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM chunk_profiles`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback incompleto: %d %v", count, err)
	}
	if _, err := s.Chunk(ctx, "inexistente"); err == nil {
		t.Fatal("extração inexistente aceita")
	}
}
