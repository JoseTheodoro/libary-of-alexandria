package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchResolvesCanonicalEvidence(t *testing.T) {
	ctx := context.Background()
	source, artifact := syntheticArtifact(t)
	path := filepath.Join(t.TempDir(), "db.sqlite")
	s, err := Init(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := s.Import(ctx, source, artifact, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := s.Chunk(ctx, imported.ExtractionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Activate(ctx, source.ID, imported.ExtractionID, chunks.ProfileID); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	verified, err := s.Verify(ctx)
	if err != nil || verified.Fragments != 1 {
		t.Fatalf("%+v %v", verified, err)
	}
	hits, err := s.Search(ctx, "Ação ReadFile", "", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("%+v %v", hits, err)
	}
	hit := hits[0]
	if hit.Text != "Ação 📚\nfunc ReadFile() {\n    return value\n}\n" || hit.Locator.Page == nil || *hit.Locator.Page != 1 || hit.StartByte != 0 || hit.EndByte != len(hit.Text) || hit.SourceID != source.ID || hit.SourceSHA256 != source.SHA256 || len(hit.Diagnostics) != 1 {
		t.Fatalf("evidência inválida: %+v", hit)
	}
	for _, q := range []string{`ReadFile OR`, `"ReadFile"`, `unknown`, `+`, `ReadFile NOT value`} {
		if _, err := s.Search(ctx, q, "", 5); err != nil {
			t.Fatalf("consulta literal %q: %v", q, err)
		}
	}
	hits, err = s.Search(ctx, "ReadFile", "other", 5)
	if err != nil || len(hits) != 0 {
		t.Fatalf("filtro: %+v %v", hits, err)
	}
	for _, q := range []string{"", "   ", "hello\x00world"} {
		if _, err := s.Search(ctx, q, "", 5); err == nil {
			t.Fatalf("consulta inválida aceita: %q", q)
		}
	}
	if _, err := s.Search(ctx, "test", "", 0); err == nil {
		t.Fatal("limite inválido aceito")
	}
}

func TestSearchRejectsStaleIndexAndInvalidLocator(t *testing.T) {
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
	chunks, err := s.Chunk(ctx, imported.ExtractionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Activate(ctx, source.ID, imported.ExtractionID, chunks.ProfileID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE document_units SET locator_json=json_set(locator_json,'$.page',2)`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Search(ctx, "ReadFile", "", 5); err == nil {
		t.Fatal("localizador inválido aceito")
	}
	if _, err = s.Verify(ctx); err == nil {
		t.Fatal("verificação aceitou página incompatível")
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE document_units SET locator_json=json_set(locator_json,'$.page',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM fragment_fts`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Search(ctx, "unknown", "", 5); err == nil {
		t.Fatal("índice ausente tratado como busca sem resultado")
	}
}

func TestOpenReadOnlyDoesNotCreateOrMigrate(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "absent.sqlite")
	if _, err := OpenReadOnly(ctx, path); err == nil {
		t.Fatal("banco ausente aceito")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("banco criado: %v", err)
	}
	s, err := Init(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=2`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReadOnly(ctx, path); err == nil {
		t.Fatal("schema antigo aceito")
	}
}

func TestSearchBreaksTiesByStableFragmentID(t *testing.T) {
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
	if _, err := s.db.ExecContext(ctx, `INSERT INTO document_units(extraction_id,unit_id,ordinal,text,locator_json) SELECT extraction_id,'page-000002',2,text,json_set(locator_json,'$.page',2) FROM document_units WHERE extraction_id=?`, imported.ExtractionID); err != nil {
		t.Fatal(err)
	}
	chunks, err := s.Chunk(ctx, imported.ExtractionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Activate(ctx, source.ID, imported.ExtractionID, chunks.ProfileID); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(ctx, "ReadFile", "", 5)
	if err != nil || len(hits) != 2 {
		t.Fatalf("%+v %v", hits, err)
	}
	if hits[0].Score != hits[1].Score || hits[0].FragmentID >= hits[1].FragmentID {
		t.Fatalf("desempate não determinístico: %+v", hits)
	}
}
