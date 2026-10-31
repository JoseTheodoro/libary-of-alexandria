package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreInitCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.sqlite")
	for i := 0; i < 2; i++ {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"store", "init", "--db", path}, &stdout, &stderr); code != 0 {
			t.Fatalf("code=%d, stderr=%s", code, &stderr)
		}
		var report struct {
			SchemaVersion int    `json:"schema_version"`
			SQLiteVersion string `json:"sqlite_version"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil || report.SchemaVersion != 2 || report.SQLiteVersion == "" {
			t.Fatalf("report=%s, err=%v", &stdout, err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"store", "init"}, &stdout, &stderr); code != 2 {
		t.Fatalf("caminho ausente: code=%d", code)
	}
}

func TestStoreImportRequiresExplicitExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"store", "import", "--db", path, "--catalog", "file", "--source", "id", "--artifact", "artifact"}, &stdout, &stderr); code != 1 {
		t.Fatalf("banco ausente: code=%d stderr=%s", code, &stderr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("importação criou banco implicitamente: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"store", "import", "--db", path}, &stdout, &stderr); code != 2 {
		t.Fatalf("argumentos ausentes: code=%d", code)
	}
}
