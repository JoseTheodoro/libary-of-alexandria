// Package catalog validates local source manifests without extracting documents.
package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/strictjson"
)

const SchemaVersion = 1

// Catalog describes sources explicitly supplied by the operator.
type Catalog struct {
	SchemaVersion int      `json:"schema_version"`
	Sources       []Source `json:"sources"`
}

// Source keeps identity separate from the SHA-256 of one file revision.
// Edition is nil when unknown; metadata is never inferred from filenames.
type Source struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Authors    []string   `json:"authors"`
	Edition    *string    `json:"edition"`
	Format     string     `json:"format"`
	SHA256     string     `json:"sha256"`
	Path       string     `json:"path"`
	Permission Permission `json:"permission"`
}

// Permission records an operator's assertion and its reference, not a legal ruling.
type Permission struct {
	Status    string `json:"status"`
	Basis     string `json:"basis"`
	Reference string `json:"reference"`
}

type Issue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Report struct {
	SchemaVersion   int     `json:"schema_version"`
	Valid           bool    `json:"valid"`
	SourceCount     int     `json:"source_count"`
	VerifiedSources int     `json:"verified_sources"`
	Issues          []Issue `json:"issues"`
}

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	formatPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Decode rejects ambiguous or unrecognized JSON rather than silently losing data.
func Decode(r io.Reader) (Catalog, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Catalog{}, fmt.Errorf("ler catálogo: %w", err)
	}
	var c Catalog
	if err := strictjson.Decode(data, &c); err != nil {
		return Catalog{}, fmt.Errorf("decodificar catálogo: %w", err)
	}
	return c, nil
}

// Checksum streams a nonempty regular file; it neither extracts nor modifies it.
func Checksum(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", fmt.Errorf("a fonte deve ser um arquivo regular não vazio")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Validate resolves relative paths from the manifest's directory, never the cwd.
// requiredCount == 0 accepts any nonempty catalog; M1 acceptance uses nine.
// Sources with incomplete or unauthorized metadata are not opened.
func Validate(c Catalog, directory string, requiredCount int) Report {
	report := Report{SchemaVersion: SchemaVersion, SourceCount: len(c.Sources), Issues: []Issue{}}
	add := func(field, code, message string) {
		report.Issues = append(report.Issues, Issue{Field: field, Code: code, Message: message})
	}
	if c.SchemaVersion != SchemaVersion {
		add("schema_version", "unsupported_version", "versão suportada: 1")
		return report
	}
	if requiredCount < 0 {
		add("sources", "invalid_count", "a quantidade exigida não pode ser negativa")
	}
	if len(c.Sources) == 0 {
		add("sources", "empty_catalog", "informe pelo menos uma fonte")
	}
	if requiredCount > 0 && len(c.Sources) != requiredCount {
		add("sources", "source_count", fmt.Sprintf("esperadas %d fontes; recebidas %d", requiredCount, len(c.Sources)))
	}
	ids := make(map[string]int)
	hashes := make(map[string]int)
	for i, source := range c.Sources {
		before := len(report.Issues)
		field := func(name string) string { return fmt.Sprintf("sources[%d].%s", i, name) }
		required := func(name, value string) {
			if strings.TrimSpace(value) == "" {
				add(field(name), "required", "campo obrigatório não informado")
			}
		}
		if !idPattern.MatchString(source.ID) {
			add(field("id"), "invalid_id", "use letras minúsculas ASCII, números, ponto, hífen ou sublinhado; inicie com letra ou número")
		} else if previous, ok := ids[source.ID]; ok {
			add(field("id"), "duplicate_id", fmt.Sprintf("ID já usado em sources[%d]", previous))
		} else {
			ids[source.ID] = i
		}
		required("title", source.Title)
		if len(source.Authors) == 0 {
			add(field("authors"), "required", "informe pelo menos um autor")
		}
		for j, author := range source.Authors {
			required(fmt.Sprintf("authors[%d]", j), author)
		}
		if source.Edition != nil {
			required("edition", *source.Edition)
		}
		if !formatPattern.MatchString(source.Format) {
			add(field("format"), "invalid_format", "informe um identificador de formato em minúsculas, como pdf ou epub")
		}
		decoded, err := hex.DecodeString(source.SHA256)
		if err != nil || len(decoded) != sha256.Size {
			add(field("sha256"), "invalid_checksum", "informe um SHA-256 com 64 dígitos hexadecimais")
		} else {
			hash := strings.ToLower(source.SHA256)
			if previous, ok := hashes[hash]; ok {
				add(field("sha256"), "duplicate_content", fmt.Sprintf("conteúdo já declarado em sources[%d]", previous))
			} else {
				hashes[hash] = i
			}
		}
		required("path", source.Path)
		if source.Permission.Status != "authorized" {
			add(field("permission.status"), "not_authorized", "processamento não autorizado: registre status authorized e a justificativa verificável")
		}
		required("permission.basis", source.Permission.Basis)
		required("permission.reference", source.Permission.Reference)
		if len(report.Issues) != before {
			continue
		}
		path := source.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(directory, path)
		}
		actual, err := Checksum(path)
		if err != nil {
			add(field("path"), "unreadable_source", fmt.Sprintf("não foi possível verificar a fonte: %v", err))
		} else if !strings.EqualFold(actual, source.SHA256) {
			add(field("sha256"), "checksum_mismatch", "o arquivo difere da revisão registrada; revise a fonte, não substitua o hash silenciosamente")
		} else {
			report.VerifiedSources++
		}
	}
	report.Valid = len(report.Issues) == 0
	return report
}
