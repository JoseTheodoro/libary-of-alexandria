// Package extract validates and stores local document extraction artifacts.
package extract

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/strictjson"
)

const SchemaVersion = 1

type Engine struct {
	AdapterVersion string `json:"adapter_version"`
	Library        string `json:"library"`
	LibraryVersion string `json:"library_version"`
	PythonVersion  string `json:"python_version"`
}

// Locator describes a representation, not necessarily a PDF. Other kinds can
// be added with explicit contract validation when another extractor exists.
type Locator struct {
	Kind           string `json:"kind"`
	Page           *int   `json:"page,omitempty"`
	Representation string `json:"representation"`
	StartByte      int    `json:"start_byte"`
	EndByte        int    `json:"end_byte"`
}

type Unit struct {
	ID      string  `json:"id"`
	Order   int     `json:"order"`
	Text    string  `json:"text"`
	Locator Locator `json:"locator"`
}

type Section struct {
	Title string `json:"title"`
	Page  int    `json:"page"`
	Depth int    `json:"depth"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Page     *int   `json:"page,omitempty"`
}

type Summary struct {
	Status      string `json:"status"`
	Units       int    `json:"units"`
	Diagnostics int    `json:"diagnostics"`
	Errors      int    `json:"errors"`
	Sections    int    `json:"sections"`
}

type Event struct {
	SchemaVersion int         `json:"schema_version"`
	Type          string      `json:"type"`
	SourceID      string      `json:"source_id"`
	SourceSHA256  string      `json:"source_sha256"`
	PageCount     *int        `json:"page_count,omitempty"`
	Engine        *Engine     `json:"engine,omitempty"`
	Mode          string      `json:"mode,omitempty"`
	Unit          *Unit       `json:"unit,omitempty"`
	Section       *Section    `json:"section,omitempty"`
	Diagnostic    *Diagnostic `json:"diagnostic,omitempty"`
	End           *Summary    `json:"end,omitempty"`
}

// Validate streams bounded JSONL records. It never repairs or normalizes text.
func Validate(input io.Reader, output io.Writer, sourceID, sourceSHA, mode string, engine Engine, maxRecord int) (Summary, error) {
	var result Summary
	if maxRecord < 1 {
		return result, fmt.Errorf("limite de registro inválido")
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, min(64*1024, maxRecord)), maxRecord)
	started, ended, pages := false, false, 0
	for scanner.Scan() {
		line := scanner.Bytes()
		var e Event
		if err := strictjson.Decode(line, &e); err != nil {
			return result, fmt.Errorf("registro JSONL inválido: %w", err)
		}
		if e.SchemaVersion != SchemaVersion || e.SourceID != sourceID || e.SourceSHA256 != sourceSHA || ended {
			return result, fmt.Errorf("identidade, versão ou sequência de eventos inválida")
		}
		fields := 0
		for _, present := range []bool{e.PageCount != nil, e.Engine != nil, e.Mode != "", e.Unit != nil, e.Section != nil, e.Diagnostic != nil, e.End != nil} {
			if present {
				fields++
			}
		}
		if !started && e.Type != "document_start" {
			return result, fmt.Errorf("document_start ausente")
		}
		switch e.Type {
		case "document_start":
			if started || fields != 3 || e.PageCount == nil || *e.PageCount < 0 || e.Engine == nil || *e.Engine != engine || e.Mode != mode {
				return result, fmt.Errorf("início de documento inválido")
			}
			pages, started = *e.PageCount, true
		case "unit":
			if fields != 1 || e.Unit == nil {
				return result, fmt.Errorf("unidade ausente")
			}
			u := e.Unit
			expected := result.Units + 1
			if expected > pages || u.Order != expected || u.ID != fmt.Sprintf("page-%06d", expected) || u.Locator.Kind != "pdf_page" || u.Locator.Page == nil || *u.Locator.Page != expected || u.Locator.Representation != "unit_text_utf8" || u.Locator.StartByte != 0 || u.Locator.EndByte != len(u.Text) {
				return result, fmt.Errorf("unidade, ordem ou localizador inválido")
			}
			result.Units++
		case "section":
			if fields != 1 || e.Section == nil || strings.TrimSpace(e.Section.Title) == "" || e.Section.Page < 1 || e.Section.Page > pages || e.Section.Depth < 0 {
				return result, fmt.Errorf("seção inválida")
			}
			result.Sections++
		case "diagnostic":
			if fields != 1 || e.Diagnostic == nil {
				return result, fmt.Errorf("diagnóstico ausente")
			}
			d := e.Diagnostic
			if d.Code == "" || d.Message == "" || (d.Severity != "warning" && d.Severity != "error") || (d.Page != nil && (*d.Page < 1 || *d.Page > pages)) {
				return result, fmt.Errorf("diagnóstico inválido")
			}
			result.Diagnostics++
			if d.Severity == "error" {
				result.Errors++
			}
		case "document_end":
			if fields != 1 || e.End == nil || result.Units != pages {
				return result, fmt.Errorf("fim ausente ou páginas não contabilizadas")
			}
			result.Status = "complete"
			if result.Errors > 0 {
				result.Status = "partial"
			}
			if *e.End != result {
				return result, fmt.Errorf("contagens ou estado final incompatíveis")
			}
			ended = true
		default:
			return result, fmt.Errorf("tipo de evento desconhecido")
		}
		if _, err := output.Write(append(append([]byte{}, line...), '\n')); err != nil {
			return result, err
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("ler registro JSONL: %w", err)
	}
	if !ended {
		return result, fmt.Errorf("document_end ausente")
	}
	return result, nil
}
