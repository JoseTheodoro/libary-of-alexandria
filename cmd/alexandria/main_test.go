package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
)

func TestCatalogCLI(t *testing.T) {
	manifest := filepath.Join("..", "..", "testdata", "catalog", "valid.json")
	var stdout, stderr bytes.Buffer
	code := run([]string{"catalog", "validate", manifest}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d, stderr=%s, stdout=%s", code, &stderr, &stdout)
	}
	var report catalog.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Valid || report.VerifiedSources != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	first := stdout.String()
	stdout.Reset()
	if code := run([]string{"catalog", "validate", manifest}, &stdout, &stderr); code != 0 || stdout.String() != first {
		t.Fatal("CLI results are not deterministic")
	}
	stdout.Reset()
	if code := run([]string{"catalog", "validate", "--require-count", "9", manifest}, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "source_count") {
		t.Fatalf("synthetic example incorrectly meets M1: code=%d, out=%s", code, &stdout)
	}
}

func TestCLIExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
	}{
		{"help", []string{"--help"}, 0},
		{"validate help", []string{"catalog", "validate", "--help"}, 0},
		{"no command", nil, 2},
		{"unknown command", []string{"search", "test"}, 2},
		{"unknown catalog command", []string{"catalog", "import"}, 2},
		{"missing path", []string{"catalog", "validate"}, 2},
		{"extra path", []string{"catalog", "validate", "one", "two"}, 2},
		{"invalid flag", []string{"catalog", "validate", "--unknown"}, 2},
		{"invalid count", []string{"catalog", "validate", "--require-count", "-1", "file"}, 2},
		{"missing catalog", []string{"catalog", "validate", filepath.Join(t.TempDir(), "missing.json")}, 1},
		{"checksum missing arg", []string{"catalog", "checksum"}, 2},
		{"checksum missing file", []string{"catalog", "checksum", filepath.Join(t.TempDir(), "missing.txt")}, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(test.args, &stdout, &stderr); code != test.code {
				t.Fatalf("code=%d, expected=%d; stderr=%s", code, test.code, &stderr)
			}
			if test.code != 0 && (stderr.Len() == 0 || stdout.Len() != 0) {
				t.Fatalf("wrong output channel: stdout=%s stderr=%s", &stdout, &stderr)
			}
		})
	}
}

func TestMalformedCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(`{"sources":`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"catalog", "validate", path}, &stdout, &stderr); code != 1 || stderr.Len() == 0 || stdout.Len() != 0 {
		t.Fatalf("code=%d, stdout=%s, stderr=%s", code, &stdout, &stderr)
	}
}

func TestChecksumCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"catalog", "checksum", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d, stderr=%s", code, &stderr)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad\n"
	if stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("stdout=%q, stderr=%s", stdout.String(), &stderr)
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestOutputFailure(t *testing.T) {
	for _, args := range [][]string{
		{"catalog", "validate", "../../testdata/catalog/valid.json"},
		{"catalog", "checksum", "../../testdata/catalog/source.txt"},
	} {
		var stderr bytes.Buffer
		if code := run(args, failedWriter{}, &stderr); code != 1 || stderr.Len() == 0 {
			t.Fatalf("output failure reported success: %d", code)
		}
	}
}
