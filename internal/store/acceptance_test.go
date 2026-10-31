package store

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEvaluateAcceptanceScoresAndFreezesGold(t *testing.T) {
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
	index, err := s.Activate(ctx, source.ID, imported.ExtractionID, chunks.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	first := AcceptanceCase{ID: "case-1", Query: "ReadFile", SourceID: source.ID, SourceSHA256: source.SHA256, ExtractionID: imported.ExtractionID, UnitID: "page-000001", StartByte: 0, EndByte: len("Ação")}
	second := first
	second.ID = "case-2"
	second.Query = "unfindableterm"
	suite := AcceptanceSuite{SchemaVersion: 1, IndexDigest: index.Digest, Cases: []AcceptanceCase{first, second}}
	report, err := s.evaluate(ctx, suite, 1, 2, 1)
	if err != nil || !report.Passed || report.Hits != 1 || report.ReferencesChecked != 1 || report.Cases[0].Rank != 1 || report.Cases[1].Rank != 0 {
		t.Fatalf("relatório incorreto: %+v %v", report, err)
	}
	again, err := s.evaluate(ctx, suite, 1, 2, 1)
	if err != nil || !reflect.DeepEqual(report, again) {
		t.Fatalf("repetição divergiu: %+v %v", again, err)
	}
	failed, err := s.evaluate(ctx, suite, 1, 2, 2)
	if err != nil || failed.Passed {
		t.Fatalf("gate incorreto: %+v %v", failed, err)
	}
	mutations := []func(*AcceptanceSuite){
		func(s *AcceptanceSuite) { s.IndexDigest = "wrong" },
		func(s *AcceptanceSuite) { s.Cases[1].ID = s.Cases[0].ID },
		func(s *AcceptanceSuite) { s.Cases[0].StartByte = 2 },
		func(s *AcceptanceSuite) { s.Cases[0].EndByte = 10000 },
		func(s *AcceptanceSuite) { s.Cases[0].SourceSHA256 = "wrong" },
		func(s *AcceptanceSuite) { s.Cases[0].Query = "" },
		func(s *AcceptanceSuite) { s.Cases[0].UnitID = "missing" },
	}
	for i, mutate := range mutations {
		changed := suite
		changed.Cases = append([]AcceptanceCase(nil), suite.Cases...)
		mutate(&changed)
		if _, err := s.evaluate(ctx, changed, 1, 2, 1); err == nil {
			t.Fatalf("gabarito inválido %d aceito", i)
		}
	}
	if _, err := s.EvaluateAcceptance(ctx, suite); err == nil {
		t.Fatal("gabarito incompleto de M1 aceito")
	}
}

func TestAcceptanceGateWithNineSyntheticSources(t *testing.T) {
	ctx := context.Background()
	s, err := Init(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var cases []AcceptanceCase
	var index IndexResult
	for i := 0; i < 9; i++ {
		source := fmt.Sprintf("source-%d", i)
		extraction := fmt.Sprintf("revision-%d", i)
		text := fmt.Sprintf("sourceword%d anothertoken%d", i, i)
		locator := fmt.Sprintf(`{"kind":"pdf_page","page":1,"representation":"unit_text_utf8","start_byte":0,"end_byte":%d}`, len(text))
		if _, err := s.db.ExecContext(ctx, `INSERT INTO sources(id,title,authors_json,format,permission_json) VALUES(?,?, '[]','pdf','{}')`, source, source); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO source_revisions(source_id,sha256) VALUES(?,?)`, source, "sha"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO extraction_revisions(id,source_id,source_sha256,events_sha256,contract_version,engine_json,mode,status) VALUES(?,?,?,'events',1,'{}','layout','complete')`, extraction, source, "sha"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO document_units(extraction_id,unit_id,ordinal,text,locator_json) VALUES(?,'page-000001',1,?,?)`, extraction, text, locator); err != nil {
			t.Fatal(err)
		}
		chunks, err := s.Chunk(ctx, extraction)
		if err != nil {
			t.Fatal(err)
		}
		index, err = s.Activate(ctx, source, extraction, chunks.ProfileID)
		if err != nil {
			t.Fatal(err)
		}
		for n, query := range []string{fmt.Sprintf("sourceword%d", i), fmt.Sprintf("anothertoken%d", i)} {
			cases = append(cases, AcceptanceCase{ID: fmt.Sprintf("q-%d-%d", i, n), Query: query, SourceID: source, SourceSHA256: "sha", ExtractionID: extraction, UnitID: "page-000001", StartByte: 0, EndByte: len(text)})
		}
	}
	suite := AcceptanceSuite{SchemaVersion: 1, IndexDigest: index.Digest, Cases: cases}
	report, err := s.EvaluateAcceptance(ctx, suite)
	if err != nil || !report.Passed || report.Hits != 18 || report.ReferencesChecked != 18 {
		t.Fatalf("gate sintético: %+v %v", report, err)
	}
	for i := 0; i < 4; i++ {
		suite.Cases[i].Query = "notpresent"
	}
	report, err = s.EvaluateAcceptance(ctx, suite)
	if err != nil || report.Passed || report.Hits != 14 {
		t.Fatalf("gate abaixo da meta: %+v %v", report, err)
	}
}
