package store

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/extract"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/strictjson"
)

type ImportResult struct {
	SourceID        string `json:"source_id"`
	ExtractionID    string `json:"extraction_id"`
	Units           int    `json:"units"`
	Sections        int    `json:"sections"`
	Diagnostics     int    `json:"diagnostics"`
	AlreadyImported bool   `json:"already_imported"`
}

// Import validates and stores a complete extraction without changing the active
// search projection. All writes are atomic and repeat imports verify stored data.
func (s *Store) Import(ctx context.Context, source catalog.Source, directory string, maxRecord int) (ImportResult, error) {
	result := ImportResult{SourceID: source.ID}
	if report := catalog.Validate(catalog.Catalog{SchemaVersion: catalog.SchemaVersion, Sources: []catalog.Source{source}}, ".", 1); !report.Valid {
		return result, fmt.Errorf("fonte não passou na validação de catálogo")
	}
	source.SHA256 = strings.ToLower(source.SHA256)
	manifest, file, err := extract.OpenVerified(source, directory, maxRecord)
	if err != nil {
		return result, fmt.Errorf("artefato recusado: %w", err)
	}
	defer file.Close()
	result.ExtractionID = manifest.Key
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	authors, _ := json.Marshal(source.Authors)
	permission, _ := json.Marshal(source.Permission)
	engine, _ := json.Marshal(manifest.Engine)
	var edition any
	if source.Edition != nil {
		edition = *source.Edition
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sources(id,title,authors_json,edition,format,permission_json) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, source.ID, source.Title, string(authors), edition, source.Format, string(permission)); err != nil {
		return result, err
	}
	var title, storedAuthors, format, storedPermission string
	var storedEdition sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT title,authors_json,edition,format,permission_json FROM sources WHERE id=?`, source.ID).Scan(&title, &storedAuthors, &storedEdition, &format, &storedPermission); err != nil {
		return result, err
	}
	if title != source.Title || storedAuthors != string(authors) || format != source.Format || storedPermission != string(permission) || storedEdition.Valid != (source.Edition != nil) || (source.Edition != nil && storedEdition.String != *source.Edition) {
		return result, fmt.Errorf("metadados de fonte existente divergentes")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO source_revisions(source_id,sha256) VALUES(?,?) ON CONFLICT DO NOTHING`, source.ID, source.SHA256); err != nil {
		return result, err
	}
	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM extraction_revisions WHERE id=?`, manifest.Key).Scan(&existing); err != nil {
		return result, err
	}
	result.AlreadyImported = existing == 1
	if _, err = tx.ExecContext(ctx, `INSERT INTO extraction_revisions(id,source_id,source_sha256,events_sha256,contract_version,engine_json,mode,status,adapter_sha256) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, manifest.Key, source.ID, source.SHA256, manifest.EventsSHA256, manifest.SchemaVersion, string(engine), manifest.Mode, manifest.Summary.Status, manifest.AdapterSHA256); err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE extraction_revisions SET adapter_sha256=? WHERE id=? AND adapter_sha256 IS NULL`, manifest.AdapterSHA256, manifest.Key); err != nil {
		return result, err
	}
	var sourceID, sourceSHA, eventsSHA, storedEngine, mode, status string
	var adapterSHA sql.NullString
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT source_id,source_sha256,events_sha256,contract_version,engine_json,mode,status,adapter_sha256 FROM extraction_revisions WHERE id=?`, manifest.Key).Scan(&sourceID, &sourceSHA, &eventsSHA, &version, &storedEngine, &mode, &status, &adapterSHA); err != nil {
		return result, err
	}
	if sourceID != source.ID || sourceSHA != source.SHA256 || eventsSHA != manifest.EventsSHA256 || version != manifest.SchemaVersion || storedEngine != string(engine) || mode != manifest.Mode || status != "complete" || !adapterSHA.Valid || adapterSHA.String != manifest.AdapterSHA256 {
		return result, fmt.Errorf("revisão de extração existente divergente")
	}
	h := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(file, h))
	if maxRecord < 1 {
		return result, fmt.Errorf("limite de registro inválido")
	}
	scanner.Buffer(make([]byte, min(64*1024, maxRecord)), maxRecord)
	for scanner.Scan() {
		var event extract.Event
		if err := strictjson.Decode(scanner.Bytes(), &event); err != nil {
			return result, err
		}
		switch event.Type {
		case "unit":
			if event.Unit == nil {
				return result, fmt.Errorf("unidade ausente durante importação")
			}
			u := event.Unit
			locator, _ := json.Marshal(u.Locator)
			if _, err = tx.ExecContext(ctx, `INSERT INTO document_units(extraction_id,unit_id,ordinal,text,locator_json) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`, manifest.Key, u.ID, u.Order, u.Text, string(locator)); err != nil {
				return result, err
			}
			var ordinal int
			var text, loc string
			if err = tx.QueryRowContext(ctx, `SELECT ordinal,text,locator_json FROM document_units WHERE extraction_id=? AND unit_id=?`, manifest.Key, u.ID).Scan(&ordinal, &text, &loc); err != nil {
				return result, fmt.Errorf("unidade conflitante: %w", err)
			}
			if ordinal != u.Order || text != u.Text || loc != string(locator) {
				return result, fmt.Errorf("unidade canônica existente divergente")
			}
			result.Units++
		case "section":
			if event.Section == nil {
				return result, fmt.Errorf("seção ausente durante importação")
			}
			result.Sections++
			v := event.Section
			locator, _ := json.Marshal(struct {
				Kind string `json:"kind"`
				Page int    `json:"page"`
			}{"pdf_page", v.Page})
			if _, err = tx.ExecContext(ctx, `INSERT INTO sections(extraction_id,ordinal,title,depth,locator_json) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`, manifest.Key, result.Sections, v.Title, v.Depth, string(locator)); err != nil {
				return result, err
			}
			var title, loc string
			var depth int
			if err = tx.QueryRowContext(ctx, `SELECT title,depth,locator_json FROM sections WHERE extraction_id=? AND ordinal=?`, manifest.Key, result.Sections).Scan(&title, &depth, &loc); err != nil {
				return result, err
			}
			if title != v.Title || depth != v.Depth || loc != string(locator) {
				return result, fmt.Errorf("seção existente divergente")
			}
		case "diagnostic":
			if event.Diagnostic == nil {
				return result, fmt.Errorf("diagnóstico ausente durante importação")
			}
			result.Diagnostics++
			d := event.Diagnostic
			var loc any
			if d.Page != nil {
				b, _ := json.Marshal(struct {
					Kind string `json:"kind"`
					Page int    `json:"page"`
				}{"pdf_page", *d.Page})
				loc = string(b)
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO diagnostics(extraction_id,ordinal,code,severity,message,locator_json) VALUES(?,?,?,?,?,?) ON CONFLICT DO NOTHING`, manifest.Key, result.Diagnostics, d.Code, d.Severity, d.Message, loc); err != nil {
				return result, err
			}
			var code, severity, message string
			var storedLoc sql.NullString
			if err = tx.QueryRowContext(ctx, `SELECT code,severity,message,locator_json FROM diagnostics WHERE extraction_id=? AND ordinal=?`, manifest.Key, result.Diagnostics).Scan(&code, &severity, &message, &storedLoc); err != nil {
				return result, err
			}
			if code != d.Code || severity != d.Severity || message != d.Message || storedLoc.Valid != (loc != nil) || (loc != nil && storedLoc.String != loc.(string)) {
				return result, fmt.Errorf("diagnóstico existente divergente")
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return result, err
	}
	if hex.EncodeToString(h.Sum(nil)) != manifest.EventsSHA256 {
		return result, fmt.Errorf("artefato mudou durante importação")
	}
	if result.Units != manifest.Summary.Units || result.Sections != manifest.Summary.Sections || result.Diagnostics != manifest.Summary.Diagnostics {
		return result, fmt.Errorf("contagens inconsistentes")
	}
	for table, expected := range map[string]int{"document_units": result.Units, "sections": result.Sections, "diagnostics": result.Diagnostics} {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE extraction_id=?`, manifest.Key).Scan(&count); err != nil {
			return result, err
		}
		if count != expected {
			return result, fmt.Errorf("registros excedentes em %s", table)
		}
	}
	actualSource, err := catalog.Checksum(source.Path)
	if err != nil || actualSource != source.SHA256 {
		return result, fmt.Errorf("fonte alterada ou indisponível durante importação")
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
