package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/extract"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/strictjson"
)

type SearchDiagnostic struct {
	Code     string          `json:"code"`
	Severity string          `json:"severity"`
	Message  string          `json:"message"`
	Locator  json.RawMessage `json:"locator,omitempty"`
}

type SearchHit struct {
	SourceID     string             `json:"source_id"`
	Title        string             `json:"title"`
	Authors      []string           `json:"authors"`
	Edition      *string            `json:"edition"`
	SourceSHA256 string             `json:"source_sha256"`
	ExtractionID string             `json:"extraction_id"`
	FragmentID   string             `json:"fragment_id"`
	UnitID       string             `json:"unit_id"`
	Locator      extract.Locator    `json:"locator"`
	StartByte    int                `json:"start_byte"`
	EndByte      int                `json:"end_byte"`
	Text         string             `json:"text"`
	Score        float64            `json:"score"`
	Diagnostics  []SearchDiagnostic `json:"diagnostics"`
}

func parseLocator(raw, text string) (extract.Locator, error) {
	var loc extract.Locator
	if err := strictjson.Decode([]byte(raw), &loc); err != nil {
		return loc, fmt.Errorf("localizador inválido: %w", err)
	}
	if loc.Kind != "pdf_page" || loc.Page == nil || *loc.Page < 1 || loc.Representation != "unit_text_utf8" || loc.StartByte != 0 || loc.EndByte != len(text) {
		return loc, fmt.Errorf("localizador inválido")
	}
	return loc, nil
}

// Verify checks the canonical fragment projection and index state without writing.
func (s *Store) Verify(ctx context.Context) (IndexResult, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return IndexResult{}, err
	}
	defer tx.Rollback()
	return inspectIndex(ctx, tx, false)
}

// Search resolves matches through canonical tables within the verified snapshot.
func (s *Store) Search(ctx context.Context, query, source string, limit int) ([]SearchHit, error) {
	if !utf8.ValidString(query) || strings.ContainsRune(query, '\x00') || len(strings.Fields(query)) == 0 || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("consulta ou limite inválido")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := inspectIndex(ctx, tx, false); err != nil {
		return nil, fmt.Errorf("índice inválido: %w", err)
	}
	return searchTx(ctx, tx, query, source, limit)
}

func searchTx(ctx context.Context, tx *sql.Tx, query, source string, limit int) ([]SearchHit, error) {
	if !utf8.ValidString(query) || strings.ContainsRune(query, '\x00') || len(strings.Fields(query)) == 0 || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("consulta ou limite inválido")
	}
	terms := strings.Fields(query)
	for i, term := range terms {
		terms[i] = `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
	}
	expression := strings.Join(terms, " AND ")
	rows, err := tx.QueryContext(ctx, `SELECT s.id,s.title,s.authors_json,s.edition,e.source_sha256,e.id,f.stable_id,u.unit_id,u.locator_json,f.start_byte,f.end_byte,u.text,bm25(fragment_fts,1,1)
		FROM fragment_fts JOIN fragments f ON f.id=fragment_fts.rowid
		JOIN active_documents a ON a.extraction_id=f.extraction_id AND a.profile_id=f.profile_id
		JOIN extraction_revisions e ON e.id=a.extraction_id AND e.source_id=a.source_id
		JOIN sources s ON s.id=a.source_id JOIN document_units u ON u.extraction_id=f.extraction_id AND u.unit_id=f.unit_id
		WHERE fragment_fts MATCH ? AND (?='' OR s.id=?) ORDER BY bm25(fragment_fts,1,1),f.stable_id LIMIT ?`, expression, source, source, limit)
	if err != nil {
		return nil, err
	}
	var hits = []SearchHit{}
	for rows.Next() {
		var hit SearchHit
		var authors, locator, text string
		var edition sql.NullString
		if err := rows.Scan(&hit.SourceID, &hit.Title, &authors, &edition, &hit.SourceSHA256, &hit.ExtractionID, &hit.FragmentID, &hit.UnitID, &locator, &hit.StartByte, &hit.EndByte, &text, &hit.Score); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(authors), &hit.Authors); err != nil {
			rows.Close()
			return nil, fmt.Errorf("autoria inválida: %w", err)
		}
		if edition.Valid {
			hit.Edition = &edition.String
		}
		hit.Locator, err = parseLocator(locator, text)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("localizador inválido: %w", err)
		}
		if hit.StartByte < 0 || hit.EndByte > len(text) || hit.EndByte <= hit.StartByte || !utf8.ValidString(text[:hit.StartByte]) || !utf8.ValidString(text[:hit.EndByte]) {
			rows.Close()
			return nil, fmt.Errorf("localizador ou intervalo inválido")
		}
		hit.Text = text[hit.StartByte:hit.EndByte]
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	cache := map[string][]SearchDiagnostic{}
	for i := range hits {
		id := hits[i].ExtractionID
		diagnostics, ok := cache[id]
		if !ok {
			diagnostics = []SearchDiagnostic{}
			drows, err := tx.QueryContext(ctx, `SELECT code,severity,message,locator_json FROM diagnostics WHERE extraction_id=? ORDER BY ordinal`, id)
			if err != nil {
				return nil, err
			}
			for drows.Next() {
				var d SearchDiagnostic
				var locator sql.NullString
				if err := drows.Scan(&d.Code, &d.Severity, &d.Message, &locator); err != nil {
					drows.Close()
					return nil, err
				}
				if locator.Valid {
					if !json.Valid([]byte(locator.String)) {
						drows.Close()
						return nil, fmt.Errorf("localizador de diagnóstico inválido")
					}
					d.Locator = json.RawMessage(locator.String)
				}
				diagnostics = append(diagnostics, d)
			}
			if err := drows.Err(); err != nil {
				drows.Close()
				return nil, err
			}
			if err := drows.Close(); err != nil {
				return nil, err
			}
			cache[id] = diagnostics
		}
		hits[i].Diagnostics = diagnostics
	}
	return hits, nil
}
