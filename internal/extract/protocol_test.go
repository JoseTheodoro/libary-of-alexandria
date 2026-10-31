package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

var testEngine = Engine{"1", "pypdf", "6.20.1", "3.12.3"}

const testSHA = "0000000000000000000000000000000000000000000000000000000000000000"

func ptr(i int) *int { return &i }
func records(text string) []Event {
	common := func(kind string) Event {
		return Event{SchemaVersion: 1, Type: kind, SourceID: "synthetic", SourceSHA256: testSHA}
	}
	start := common("document_start")
	start.PageCount = ptr(1)
	start.Engine = &testEngine
	start.Mode = "layout"
	unit := common("unit")
	unit.Unit = &Unit{ID: "page-000001", Order: 1, Text: text, Locator: Locator{"pdf_page", ptr(1), "unit_text_utf8", 0, len(text)}}
	end := common("document_end")
	end.End = &Summary{Status: "complete", Units: 1}
	return []Event{start, unit, end}
}
func stream(events []Event) string {
	var b bytes.Buffer
	for _, e := range events {
		if err := json.NewEncoder(&b).Encode(e); err != nil {
			panic(err)
		}
	}
	return b.String()
}

func TestProtocolPreservesLargeMultilingualText(t *testing.T) {
	text := strings.Repeat("Ação 📚 / Evidence\nfunc ReadFile() {\n    return value\n}\n", 4000)
	input := stream(records(text))
	var out bytes.Buffer
	got, err := Validate(strings.NewReader(input), &out, "synthetic", testSHA, "layout", testEngine, 2*1024*1024)
	if err != nil || got.Units != 1 || got.Status != "complete" || out.String() != input {
		t.Fatalf("summary=%+v error=%v", got, err)
	}
}

func TestProtocolRejectsInvalidEvents(t *testing.T) {
	tests := []struct {
		name   string
		change func([]Event) []Event
	}{
		{"missing start", func(e []Event) []Event { return e[1:] }},
		{"missing end", func(e []Event) []Event { return e[:2] }},
		{"extra event", func(e []Event) []Event { return append(e, e[1]) }},
		{"version", func(e []Event) []Event { e[0].SchemaVersion = 2; return e }},
		{"identity", func(e []Event) []Event { e[1].SourceID = "other"; return e }},
		{"revision", func(e []Event) []Event { e[1].SourceSHA256 = "other"; return e }},
		{"engine", func(e []Event) []Event { e[0].Engine = &Engine{}; return e }},
		{"mode", func(e []Event) []Event { e[0].Mode = "plain"; return e }},
		{"duplicate start", func(e []Event) []Event { return append(e[:1], e...) }},
		{"duplicate unit", func(e []Event) []Event { return []Event{e[0], e[1], e[1], e[2]} }},
		{"wrong order", func(e []Event) []Event { e[1].Unit.Order = 3; return e }},
		{"wrong page", func(e []Event) []Event { e[1].Unit.Locator.Page = ptr(2); return e }},
		{"missing page", func(e []Event) []Event { e[1].Unit.Locator.Page = nil; return e }},
		{"invalid span", func(e []Event) []Event { e[1].Unit.Locator.EndByte = 3; return e }},
		{"counts", func(e []Event) []Event { e[2].End.Units = 0; return e }},
		{"inconsistent status", func(e []Event) []Event { e[2].End.Status = "partial"; return e }},
		{"unknown type", func(e []Event) []Event { e[1].Type = "unknown"; return e }},
		{"unexpected payload", func(e []Event) []Event { e[1].Section = &Section{Title: "bad", Page: 1}; return e }},
		{"bad section", func(e []Event) []Event {
			e[1].Type = "section"
			e[1].Unit = nil
			e[1].Section = &Section{Title: "bad", Page: 2}
			return e
		}},
		{"bad diagnostic", func(e []Event) []Event {
			e[1].Type = "diagnostic"
			e[1].Unit = nil
			e[1].Diagnostic = &Diagnostic{Code: "bad", Message: "bad", Severity: "ignore"}
			return e
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Validate(strings.NewReader(stream(test.change(records("ação")))), io.Discard, "synthetic", testSHA, "layout", testEngine, 1<<20)
			if err == nil {
				t.Fatal("invalid protocol accepted")
			}
		})
	}
	for _, input := range []string{"not json\n", strings.Replace(stream(records("ok")), `"text":"ok"`, `"text":"\ud800"`, 1), strings.Replace(stream(records("ok")), `"text":"ok"`, `"text":"ok","text":"other"`, 1)} {
		if _, err := Validate(strings.NewReader(input), io.Discard, "synthetic", testSHA, "layout", testEngine, 1<<20); err == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
	if _, err := Validate(strings.NewReader(stream(records(strings.Repeat("x", 10000)))), io.Discard, "synthetic", testSHA, "layout", testEngine, 512); err == nil {
		t.Fatal("record limit ignored")
	}
}

func TestProtocolAccountsDiagnostics(t *testing.T) {
	e := records("")
	d := e[0]
	d.Type = "diagnostic"
	d.PageCount = nil
	d.Engine = nil
	d.Mode = ""
	d.Diagnostic = &Diagnostic{Code: "page_error", Message: "unreadable page", Severity: "error", Page: ptr(1)}
	e[2].End = &Summary{Status: "partial", Units: 1, Diagnostics: 1, Errors: 1}
	summary, err := Validate(strings.NewReader(stream([]Event{e[0], d, e[1], e[2]})), io.Discard, "synthetic", testSHA, "layout", testEngine, 1<<20)
	if err != nil || summary.Status != "partial" {
		t.Fatalf("%+v %v", summary, err)
	}
}

func TestProtocolWriterFailure(t *testing.T) {
	_, err := Validate(strings.NewReader(stream(records("ok"))), brokenWriter{}, "synthetic", testSHA, "layout", testEngine, 1<<20)
	if err == nil {
		t.Fatal("write failure ignored")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("write failed") }
