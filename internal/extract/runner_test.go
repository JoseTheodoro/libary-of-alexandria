package extract

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
)

func runnerFixture(t *testing.T, behavior string) (catalog.Source, Options) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python required for subprocess integration")
	}
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.pdf")
	if err = os.WriteFile(sourcePath, []byte("synthetic source; not a real book"), 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := catalog.Checksum(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	source := catalog.Source{ID: "synthetic", Title: "Synthetic", Authors: []string{"Test"}, Format: "pdf", Path: sourcePath, SHA256: digest, Permission: catalog.Permission{Status: "authorized", Basis: "Synthetic fixture", Reference: "test"}}
	engine, _ := json.Marshal(testEngine)
	script := `import json,sys,time
engine=json.loads(` + "'" + string(engine) + "'" + `)
if sys.argv[1:]==['--info']:
 print(json.dumps(engine));sys.exit(0)
r=json.load(sys.stdin)
common={'schema_version':1,'source_id':r['source_id'],'source_sha256':r['source_sha256']}
def emit(kind,**kw): print(json.dumps(dict(common,type=kind,**kw)),flush=True)
emit('document_start',page_count=1,engine=engine,mode=r['mode'])
` + behavior + `
text='Ação / ReadFile\n    return value'
emit('unit',unit={'id':'page-000001','order':1,'text':text,'locator':{'kind':'pdf_page','page':1,'representation':'unit_text_utf8','start_byte':0,'end_byte':len(text.encode('utf8'))}})
emit('document_end',end={'status':'complete','units':1,'diagnostics':0,'errors':0,'sections':0})
`
	adapter := filepath.Join(root, "adapter.py")
	if err = os.WriteFile(adapter, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	return source, Options{Python: python, Adapter: adapter, Output: filepath.Join(root, "extracted"), Mode: "layout", Timeout: 5 * time.Second, MaxRecord: 1 << 20}
}

func TestRunnerPublicationReuseAndCorruption(t *testing.T) {
	source, opts := runnerFixture(t, "")
	first, err := Run(context.Background(), source, opts)
	if err != nil || first.Reused || first.Summary.Units != 1 {
		t.Fatalf("%+v %v", first, err)
	}
	second, err := Run(context.Background(), source, opts)
	if err != nil || !second.Reused || second.Key != first.Key {
		t.Fatalf("%+v %v", second, err)
	}
	if err = os.WriteFile(filepath.Join(first.Directory, "events.jsonl"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Run(context.Background(), source, opts); err == nil {
		t.Fatal("corrupted cache reused")
	}
}

func TestRunnerModeCreatesNewRevision(t *testing.T) {
	source, opts := runnerFixture(t, "")
	a, err := Run(context.Background(), source, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Mode = "plain"
	b, err := Run(context.Background(), source, opts)
	if err != nil || a.Key == b.Key || b.Reused {
		t.Fatalf("%+v %v", b, err)
	}
}

func TestRunnerFailureTimeoutAndLock(t *testing.T) {
	for _, behavior := range []string{"print('not json',flush=True);sys.exit(1)", "time.sleep(3)", "sys.exit(1)"} {
		t.Run(behavior, func(t *testing.T) {
			source, opts := runnerFixture(t, behavior)
			opts.Timeout = 300 * time.Millisecond
			result, err := Run(context.Background(), source, opts)
			if err == nil || result.Error == "" {
				t.Fatal("failed extraction accepted")
			}
			if _, err = os.Stat(filepath.Join(result.Directory, "manifest.json")); err == nil {
				t.Fatal("failure published as complete")
			}
		})
	}
	source, opts := runnerFixture(t, "")
	first, err := Run(context.Background(), source, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(first.Directory+".lock", 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = Run(context.Background(), source, opts); err == nil {
		t.Fatal("lock ignored")
	}
}

func TestRunnerRejectsUnauthorizedAndChangedSource(t *testing.T) {
	source, opts := runnerFixture(t, "")
	source.Permission.Status = "unknown"
	if _, err := Run(context.Background(), source, opts); err == nil {
		t.Fatal("unauthorized source accepted")
	}
	source.Permission.Status = "authorized"
	source.SHA256 = strings.Repeat("0", 64)
	if _, err := Run(context.Background(), source, opts); err == nil {
		t.Fatal("changed source accepted")
	}
}

func TestRunnerPreservesEveryFailedAttempt(t *testing.T) {
	source, opts := runnerFixture(t, "print('not json',flush=True);sys.exit(1)")
	var first Result
	for i := 0; i < 2; i++ {
		result, err := Run(context.Background(), source, opts)
		if err == nil {
			t.Fatal("failed extraction accepted")
		}
		first = result
	}
	attempts, err := filepath.Glob(first.Directory + ".failed-*")
	if err != nil || len(attempts) != 2 {
		t.Fatalf("expected two retained failures, got %v: %v", attempts, err)
	}
	for _, dir := range attempts {
		data, err := os.ReadFile(filepath.Join(dir, "failure.json"))
		if err != nil {
			t.Fatalf("missing diagnostic for %s: %v", dir, err)
		}
		var failure struct {
			Cause string `json:"cause"`
		}
		if err := json.Unmarshal(data, &failure); err != nil || failure.Cause == "" {
			t.Fatalf("missing failure cause: %s: %v", data, err)
		}
	}
}

func TestRunnerWithRealPDFAdapter(t *testing.T) {
	source, opts := runnerFixture(t, "")
	if err := exec.Command(opts.Python, "-c", "import pypdf").Run(); err != nil {
		t.Skip("pypdf required for real PDF integration; installed explicitly in CI")
	}
	adapter, err := filepath.Abs("../../adapters/pdf/extract.py")
	if err != nil {
		t.Fatal(err)
	}
	create := exec.Command(opts.Python, "-c", "import sys; sys.path.insert(0,sys.argv[1]); from test_extract import write_pdf; write_pdf(sys.argv[2])", filepath.Dir(adapter), source.Path)
	create.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := create.CombinedOutput(); err != nil {
		t.Fatalf("create fixture: %s: %v", output, err)
	}
	source.SHA256, err = catalog.Checksum(source.Path)
	if err != nil {
		t.Fatal(err)
	}
	opts.Adapter = adapter
	first, err := Run(context.Background(), source, opts)
	if err != nil || first.Summary.Units != 2 || first.Summary.Sections != 1 || first.Summary.Diagnostics < 1 {
		t.Fatalf("%+v: %v", first, err)
	}
	second, err := Run(context.Background(), source, opts)
	if err != nil || !second.Reused || first.Summary != second.Summary {
		t.Fatalf("%+v: %v", second, err)
	}
}
