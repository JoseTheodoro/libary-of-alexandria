package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAcceptanceRejectsMissingOffsetsAndDoesNotCreateDatabase(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "missing.sqlite")
	suite := filepath.Join(dir, "suite.json")
	if err := os.WriteFile(suite, []byte(`{"schema_version":1,"index_digest":"x","cases":[{"id":"x","end_byte":1}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"acceptance", "--db", db, "--suite", suite}, &stdout, &stderr); code != 1 || !bytes.Contains(stderr.Bytes(), []byte("intervalo incompleto")) {
		t.Fatalf("code %d, stderr %s", code, &stderr)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("banco criado: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"acceptance", "--db", db}, &stdout, &stderr); code != 2 {
		t.Fatalf("code %d", code)
	}
}
