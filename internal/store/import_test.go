package store

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/extract"
)

func syntheticArtifact(t *testing.T) (catalog.Source, string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python necessário para fixture de integração")
	}
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("synthetic source for tests"), 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := catalog.Checksum(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	source := catalog.Source{ID: "synthetic", Title: "Fonte sintética", Authors: []string{"Teste"}, Format: "pdf", SHA256: digest, Path: sourcePath, Permission: catalog.Permission{Status: "authorized", Basis: "Conteúdo sintético", Reference: "fixture"}}
	script := `import json,sys
engine={'adapter_version':'1','library':'pypdf','library_version':'6.20.1','python_version':'3.12.3'}
if sys.argv[1:]==['--info']:
 print(json.dumps(engine));sys.exit(0)
r=json.load(sys.stdin)
base={'schema_version':1,'source_id':r['source_id'],'source_sha256':r['source_sha256']}
def emit(kind,**data): print(json.dumps(dict(base,type=kind,**data)),flush=True)
emit('document_start',page_count=1,engine=engine,mode=r['mode'])
emit('section',section={'title':'Capítulo','page':1,'depth':0})
emit('diagnostic',diagnostic={'code':'sample','message':'diagnóstico sintético','severity':'warning','page':1})
text='Ação 📚\nfunc ReadFile() {\n    return value\n}\n'
emit('unit',unit={'id':'page-000001','order':1,'text':text,'locator':{'kind':'pdf_page','page':1,'representation':'unit_text_utf8','start_byte':0,'end_byte':len(text.encode('utf8'))}})
emit('document_end',end={'status':'complete','units':1,'diagnostics':1,'errors':0,'sections':1})
`
	adapter := filepath.Join(root, "adapter.py")
	if err := os.WriteFile(adapter, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := extract.Run(context.Background(), source, extract.Options{Python: python, Adapter: adapter, Output: filepath.Join(root, "artifacts"), Mode: "layout", Timeout: 5 * time.Second, MaxRecord: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	return source, result.Directory
}

func TestImportIsIdempotentAndPreservesCanonicalText(t *testing.T) {
	ctx := context.Background()
	source, artifact := syntheticArtifact(t)
	s, err := Init(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Import(ctx, source, artifact, 1<<20)
	if err != nil || first.AlreadyImported || first.Units != 1 || first.Sections != 1 || first.Diagnostics != 1 {
		t.Fatalf("%+v %v", first, err)
	}
	second, err := s.Import(ctx, source, artifact, 1<<20)
	if err != nil || !second.AlreadyImported || second.ExtractionID != first.ExtractionID {
		t.Fatalf("%+v %v", second, err)
	}
	var text, locator string
	if err := s.db.QueryRowContext(ctx, `SELECT text,locator_json FROM document_units`).Scan(&text, &locator); err != nil {
		t.Fatal(err)
	}
	if text != "Ação 📚\nfunc ReadFile() {\n    return value\n}\n" || !strings.Contains(locator, `"page":1`) {
		t.Fatalf("texto ou proveniência alterados: %q %q", text, locator)
	}
	var adapterSHA string
	if err := s.db.QueryRowContext(ctx, `SELECT adapter_sha256 FROM extraction_revisions`).Scan(&adapterSHA); err != nil || len(adapterSHA) != 64 {
		t.Fatalf("proveniência do adaptador ausente: %q, %v", adapterSHA, err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM document_units`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicação: %d, %v", count, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE document_units SET text='modified'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(ctx, source, artifact, 1<<20); err == nil {
		t.Fatal("conteúdo canônico alterado foi aceito")
	}
}

func TestImportRejectsCorruptArtifactAndRollsBack(t *testing.T) {
	ctx := context.Background()
	source, artifact := syntheticArtifact(t)
	s, err := Init(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_unit BEFORE INSERT ON document_units BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(ctx, source, artifact, 1<<20); err == nil {
		t.Fatal("falha de gravação não interrompeu importação")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback incompleto: %d, %v", count, err)
	}
	if _, err := s.db.ExecContext(ctx, `DROP TRIGGER fail_unit`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "events.jsonl"), []byte("corrupted\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(ctx, source, artifact, 1<<20); err == nil {
		t.Fatal("artefato corrompido aceito")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("dados gravados após corrupção: %d, %v", count, err)
	}
}

func TestImportRejectsUnapprovedSource(t *testing.T) {
	source, artifact := syntheticArtifact(t)
	source.Permission.Status = "unknown"
	s, err := Init(context.Background(), filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Import(context.Background(), source, artifact, 1<<20); err == nil {
		t.Fatal("fonte sem autorização aceita")
	}
}

func TestImportRejectsManipulatedManifest(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"wrong identity", func(m map[string]any) { m["key"] = strings.Repeat("0", 64) }},
		{"partial extraction", func(m map[string]any) { m["summary"].(map[string]any)["status"] = "partial" }},
		{"wrong digest", func(m map[string]any) { m["events_sha256"] = strings.Repeat("0", 64) }},
		{"different source", func(m map[string]any) { m["source_id"] = "another-source" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, artifact := syntheticArtifact(t)
			path := filepath.Join(artifact, "manifest.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var manifest map[string]any
			if err = json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			test.change(manifest)
			data, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := Init(context.Background(), filepath.Join(t.TempDir(), "db.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err = s.Import(context.Background(), source, artifact, 1<<20); err == nil {
				t.Fatal("manifesto manipulado aceito")
			}
			var count int
			if err = s.db.QueryRow(`SELECT COUNT(*) FROM sources`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("dados gravados: %d %v", count, err)
			}
		})
	}
}
