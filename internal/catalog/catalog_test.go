package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, count int) (Catalog, string) {
	t.Helper()
	directory := t.TempDir()
	c := Catalog{SchemaVersion: SchemaVersion}
	for i := range count {
		name := fmt.Sprintf("source-%d.txt", i)
		data := []byte(fmt.Sprintf("Fonte sintética %d — ação e evidence\nfunc ReadFile() {}\n", i))
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		edition := "Edição sintética de teste"
		c.Sources = append(c.Sources, Source{
			ID: fmt.Sprintf("synthetic-%d", i), Title: "Fonte sintética — mixed language",
			Authors: []string{"Autor sintético"}, Edition: &edition, Format: "txt",
			SHA256: hex.EncodeToString(hash[:]), Path: name,
			Permission: Permission{Status: "authorized", Basis: "Conteúdo sintético criado para o teste", Reference: "fixture gerada pelo teste"},
		})
	}
	return c, directory
}

func TestValidateNineSyntheticSources(t *testing.T) {
	c, directory := fixture(t, 9)
	before, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	first := Validate(c, directory, 9)
	second := Validate(c, directory, 9)
	if !first.Valid || first.SourceCount != 9 || first.VerifiedSources != 9 {
		t.Fatalf("unexpected report: %+v", first)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("validation is not repeatable: %+v vs %+v", first, second)
	}
	after, _ := json.Marshal(c)
	if string(before) != string(after) {
		t.Fatal("validation modified metadata")
	}
}

func TestValidateFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Catalog)
		code string
	}{
		{"version", func(c *Catalog) { c.SchemaVersion = 2 }, "unsupported_version"},
		{"empty", func(c *Catalog) { c.Sources = nil }, "empty_catalog"},
		{"invalid ID", func(c *Catalog) { c.Sources[0].ID = "invalid ID" }, "invalid_id"},
		{"duplicate ID", func(c *Catalog) { c.Sources[1].ID = c.Sources[0].ID }, "duplicate_id"},
		{"duplicate content", func(c *Catalog) { c.Sources[1].SHA256 = strings.ToUpper(c.Sources[0].SHA256) }, "duplicate_content"},
		{"title", func(c *Catalog) { c.Sources[0].Title = " " }, "required"},
		{"authors", func(c *Catalog) { c.Sources[0].Authors = nil }, "required"},
		{"empty author", func(c *Catalog) { c.Sources[0].Authors = []string{" "} }, "required"},
		{"empty edition", func(c *Catalog) { empty := ""; c.Sources[0].Edition = &empty }, "required"},
		{"format", func(c *Catalog) { c.Sources[0].Format = "" }, "invalid_format"},
		{"malformed hash", func(c *Catalog) { c.Sources[0].SHA256 = strings.Repeat("x", 64) }, "invalid_checksum"},
		{"short hash", func(c *Catalog) { c.Sources[0].SHA256 = "abcd" }, "invalid_checksum"},
		{"changed source", func(c *Catalog) { c.Sources[0].SHA256 = strings.Repeat("0", 64) }, "checksum_mismatch"},
		{"empty path", func(c *Catalog) { c.Sources[0].Path = "" }, "required"},
		{"missing source", func(c *Catalog) { c.Sources[0].Path = "absent.txt" }, "unreadable_source"},
		{"directory", func(c *Catalog) { c.Sources[0].Path = "." }, "unreadable_source"},
		{"permission absent", func(c *Catalog) { c.Sources[0].Permission = Permission{} }, "not_authorized"},
		{"permission unknown", func(c *Catalog) { c.Sources[0].Permission.Status = "unknown" }, "not_authorized"},
		{"permission denied", func(c *Catalog) { c.Sources[0].Permission.Status = "denied" }, "not_authorized"},
		{"permission basis", func(c *Catalog) { c.Sources[0].Permission.Basis = "" }, "required"},
		{"permission reference", func(c *Catalog) { c.Sources[0].Permission.Reference = "" }, "required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, directory := fixture(t, 2)
			test.edit(&c)
			report := Validate(c, directory, 0)
			if report.Valid || !hasCode(report, test.code) {
				t.Fatalf("expected %s: %+v", test.code, report)
			}
		})
	}
}

func hasCode(report Report, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestPermissionCheckedBeforeFileAccess(t *testing.T) {
	c, directory := fixture(t, 1)
	c.Sources[0].Permission.Status = "unknown"
	c.Sources[0].Path = "does-not-exist.txt"
	report := Validate(c, directory, 0)
	if !hasCode(report, "not_authorized") || hasCode(report, "unreadable_source") || report.VerifiedSources != 0 {
		t.Fatalf("unauthorized source should not be opened: %+v", report)
	}
}

func TestUnknownEditionAndAbsolutePath(t *testing.T) {
	c, directory := fixture(t, 1)
	c.Sources[0].Edition = nil
	c.Sources[0].Path = filepath.Join(directory, c.Sources[0].Path)
	c.Sources[0].SHA256 = strings.ToUpper(c.Sources[0].SHA256)
	if report := Validate(c, t.TempDir(), 1); !report.Valid {
		t.Fatalf("unknown edition, absolute paths and hex case are valid: %+v", report)
	}
}

func TestRequiredCount(t *testing.T) {
	c, directory := fixture(t, 1)
	if report := Validate(c, directory, 9); report.Valid || !hasCode(report, "source_count") {
		t.Fatalf("one fixture is not nine books: %+v", report)
	}
	if report := Validate(c, directory, -1); report.Valid || !hasCode(report, "invalid_count") {
		t.Fatalf("negative count accepted: %+v", report)
	}
}

func TestChecksumDoesNotModifyFile(t *testing.T) {
	c, directory := fixture(t, 1)
	path := filepath.Join(directory, c.Sources[0].Path)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := Checksum(path)
	if err != nil || actual != c.Sources[0].SHA256 {
		t.Fatalf("checksum=%q, err=%v", actual, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("source changed")
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Checksum(path); err == nil {
		t.Fatal("empty source accepted")
	}
}

func TestDecodePreservesMetadata(t *testing.T) {
	c, _ := fixture(t, 1)
	c.Sources[0].Title = "  Ação — ReadFile / 技術  "
	c.Sources[0].Edition = nil
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(strings.NewReader(string(data)))
	if err != nil || !reflect.DeepEqual(got, c) {
		t.Fatalf("metadata lost: %+v, %v", got, err)
	}
}

func TestDecodeRejectsAmbiguousJSON(t *testing.T) {
	for _, input := range []string{
		``, `{`, `[]`, `{"schema_version":1,"typo":true}`, `{"schema_version":1} {}`,
		`{"schema_version":1} trailing`, `{"schema_version":1,"sources":{}}`,
		`{"schema_version":1,"schema_version":2}`,
		`{"schema_version":1,"SCHEMA_VERSION":2}`,
		`{"sources":[{"permission":{"status":"denied","status":"authorized"}}]}`,
		`{"sources":[{"permission":{"status":"authorized","typo":true}}]}`,
		"{\"title\":\"\xff\"}",
		`{"sources":[{"title":"\ud800"}]}`,
		`{"sources":[{"title":"\udc00"}]}`,
		`{"sources":[{"title":"\ud800\u0041"}]}`,
		`{"sources":[{"title":"\ud800\uxxxx"}]}`,
		`{"sources":[{"title":"\u123"}]}`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := Decode(strings.NewReader(input)); err == nil {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
}

func TestDecodePreservesValidUnicodeEscapes(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{`{"sources":[{"title":"A\u00e7\u00e3o \ud83d\udcda"}]}`, "Ação 📚"},
		{`{"sources":[{"title":"literal \\ud800"}]}`, `literal \ud800`},
		{`{"sources":[{"title":"quote \" / \/"}]}`, `quote " / /`},
		{`{"sources":[{"title":"replacement \ufffd"}]}`, "replacement �"},
	} {
		got, err := Decode(strings.NewReader(test.input))
		if err != nil || len(got.Sources) != 1 || got.Sources[0].Title != test.want {
			t.Fatalf("input=%s: got=%+v, err=%v", test.input, got, err)
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(`{"schema_version":1,"sources":[]}`)
	f.Add(`{"sources":[{"permission":{"status":"denied","STATUS":"authorized"}}]}`)
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = Decode(strings.NewReader(input))
	})
}
