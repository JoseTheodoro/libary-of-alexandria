package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestActivateAndRebuildFromCanonical(t *testing.T) {
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
	if _, err := s.Activate(ctx, "wrong", imported.ExtractionID, chunks.ProfileID); err == nil {
		t.Fatal("fonte incorreta aceita")
	}
	first, err := s.Activate(ctx, source.ID, imported.ExtractionID, chunks.ProfileID)
	if err != nil || first.Documents != 1 || first.Fragments != 1 {
		t.Fatalf("%+v %v", first, err)
	}
	var text, canonical string
	if err := s.db.QueryRowContext(ctx, `SELECT body FROM fragment_fts WHERE fragment_fts MATCH ?`, `"ReadFile"`).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT text FROM document_units`).Scan(&canonical); err != nil || text != canonical {
		t.Fatalf("projeção não corresponde ao canônico: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM fragment_fts`); err != nil {
		t.Fatal(err)
	}
	second, err := s.RebuildIndex(ctx)
	if err != nil || second != first {
		t.Fatalf("reconstrução: %+v %v", second, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT body FROM fragment_fts`).Scan(&text); err != nil || text != canonical {
		t.Fatalf("reconstrução incorreta: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE document_units SET text='alterado'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RebuildIndex(ctx); err == nil {
		t.Fatal("texto corrompido foi indexado")
	}
	var digest, persisted string
	if err := s.db.QueryRowContext(ctx, `SELECT active_digest FROM index_state`).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT body FROM fragment_fts`).Scan(&persisted); err != nil || digest != first.Digest || persisted != canonical {
		t.Fatalf("rollback falhou: %v", err)
	}
}

func TestActivationFailurePreservesPreviousSelection(t *testing.T) {
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
	first, err := s.Activate(ctx, source.ID, imported.ExtractionID, chunks.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO chunk_profiles(id,config_json) VALUES('bad','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Activate(ctx, source.ID, imported.ExtractionID, "bad"); err == nil {
		t.Fatal("perfil inválido aceito")
	}
	var profile, digest string
	if err := s.db.QueryRowContext(ctx, `SELECT profile_id FROM active_documents`).Scan(&profile); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT active_digest FROM index_state`).Scan(&digest); err != nil || profile != chunks.ProfileID || digest != first.Digest {
		t.Fatalf("seleção alterada após falha: %v", err)
	}
}

func TestActivationSwitchesRevisionWithoutMixingHistory(t *testing.T) {
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
	old, err := s.Chunk(ctx, imported.ExtractionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Activate(ctx, source.ID, imported.ExtractionID, old.ProfileID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO extraction_revisions(id,source_id,source_sha256,events_sha256,contract_version,engine_json,mode,status,adapter_sha256) SELECT 'new-revision',source_id,source_sha256,events_sha256,contract_version,engine_json,mode,status,adapter_sha256 FROM extraction_revisions WHERE id=?`, imported.ExtractionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO document_units(extraction_id,unit_id,ordinal,text,locator_json) SELECT 'new-revision',unit_id,ordinal,'different revision','{"kind":"pdf_page","page":1,"representation":"unit_text_utf8","start_byte":0,"end_byte":18}' FROM document_units WHERE extraction_id=?`, imported.ExtractionID); err != nil {
		t.Fatal(err)
	}
	newChunks, err := s.Chunk(ctx, "new-revision")
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Activate(ctx, source.ID, "new-revision", newChunks.ProfileID)
	if err != nil || result.Documents != 1 || result.Fragments != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	var text string
	if err := s.db.QueryRowContext(ctx, `SELECT body FROM fragment_fts`).Scan(&text); err != nil || text != "different revision" {
		t.Fatalf("índice mistura revisões: %q %v", text, err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM fragments`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("histórico não preservado: %d %v", count, err)
	}
}
