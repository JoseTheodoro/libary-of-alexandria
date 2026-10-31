package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

const indexConfig = `{"version":1,"tokenizer":"unicode61 remove_diacritics 0","ranking":"bm25","weights":[1,1]}`

type IndexResult struct {
	Documents int    `json:"documents"`
	Fragments int    `json:"fragments"`
	Digest    string `json:"active_digest"`
}

// Activate selects a complete revision and rebuilds the projection atomically.
func (s *Store) Activate(ctx context.Context, sourceID, extractionID, profileID string) (IndexResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IndexResult{}, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM extraction_revisions WHERE source_id=? AND id=?`, sourceID, extractionID).Scan(&status); err != nil {
		return IndexResult{}, fmt.Errorf("revisão não pertence à fonte: %w", err)
	}
	if status != "complete" {
		return IndexResult{}, fmt.Errorf("extração incompleta não pode ser ativada")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO active_documents(source_id,extraction_id,profile_id) VALUES(?,?,?) ON CONFLICT(source_id) DO UPDATE SET extraction_id=excluded.extraction_id,profile_id=excluded.profile_id`, sourceID, extractionID, profileID); err != nil {
		return IndexResult{}, err
	}
	result, err := rebuildIndex(ctx, tx)
	if err != nil {
		return IndexResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return IndexResult{}, err
	}
	return result, nil
}

// RebuildIndex recreates only the derived FTS5 projection, transactionally.
func (s *Store) RebuildIndex(ctx context.Context) (IndexResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IndexResult{}, err
	}
	defer tx.Rollback()
	result, err := rebuildIndex(ctx, tx)
	if err != nil {
		return IndexResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return IndexResult{}, err
	}
	return result, nil
}

// VerifyFTS checks both the canonical projection and SQLite's internal FTS index.
// The SQLite check requires a write transaction; all changes are rolled back.
func (s *Store) VerifyFTS(ctx context.Context) (IndexResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IndexResult{}, err
	}
	defer tx.Rollback()
	result, err := inspectIndex(ctx, tx, false)
	if err != nil {
		return IndexResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO fragment_fts(fragment_fts) VALUES('integrity-check')`); err != nil {
		return IndexResult{}, fmt.Errorf("integridade interna FTS5: %w", err)
	}
	if err := tx.Rollback(); err != nil {
		return IndexResult{}, err
	}
	return result, nil
}

type activeDocument struct{ source, extraction, profile, title, config, status string }

func rebuildIndex(ctx context.Context, tx *sql.Tx) (IndexResult, error) {
	return inspectIndex(ctx, tx, true)
}

// inspectIndex shares canonical validation between rebuild and read-only verification.
func inspectIndex(ctx context.Context, tx *sql.Tx, rebuild bool) (IndexResult, error) {
	var result IndexResult
	foreignKeys, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return result, err
	}
	if foreignKeys.Next() {
		foreignKeys.Close()
		return result, fmt.Errorf("referências de banco inconsistentes")
	}
	if err := foreignKeys.Err(); err != nil {
		foreignKeys.Close()
		return result, err
	}
	if err := foreignKeys.Close(); err != nil {
		return result, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.source_id,a.extraction_id,a.profile_id,s.title,p.config_json,e.status FROM active_documents a JOIN sources s ON s.id=a.source_id JOIN chunk_profiles p ON p.id=a.profile_id JOIN extraction_revisions e ON e.id=a.extraction_id AND e.source_id=a.source_id ORDER BY a.source_id`)
	if err != nil {
		return result, err
	}
	var docs []activeDocument
	for rows.Next() {
		var d activeDocument
		if err := rows.Scan(&d.source, &d.extraction, &d.profile, &d.title, &d.config, &d.status); err != nil {
			rows.Close()
			return result, err
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	var activeCount int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM active_documents`).Scan(&activeCount); err != nil {
		return result, err
	}
	if activeCount != len(docs) {
		return result, fmt.Errorf("seleção ativa possui referências inválidas")
	}
	if rebuild {
		if _, err := tx.ExecContext(ctx, `DELETE FROM fragment_fts`); err != nil {
			return result, err
		}
	}
	h := sha256.New()
	enc := json.NewEncoder(h)
	if err := enc.Encode(indexConfig); err != nil {
		return result, err
	}
	for _, d := range docs {
		if d.status != "complete" || d.config != chunkProfile || d.profile != digest([]byte(d.config)) {
			return result, fmt.Errorf("extração ou perfil ativo inválido")
		}
		if err := enc.Encode([]string{d.source, d.extraction, d.profile, d.title}); err != nil {
			return result, err
		}
		units, err := tx.QueryContext(ctx, `SELECT u.unit_id,u.ordinal,u.text,u.locator_json,f.id,f.stable_id,f.start_byte,f.end_byte FROM document_units u LEFT JOIN fragments f ON f.extraction_id=u.extraction_id AND f.unit_id=u.unit_id AND f.profile_id=? WHERE u.extraction_id=? ORDER BY u.ordinal,f.id`, d.profile, d.extraction)
		if err != nil {
			return result, err
		}
		count := 0
		lastUnit := ""
		for units.Next() {
			var unit, text, locator string
			var ordinal int
			var id, start, end sql.NullInt64
			var stable sql.NullString
			if err := units.Scan(&unit, &ordinal, &text, &locator, &id, &stable, &start, &end); err != nil {
				units.Close()
				return result, err
			}
			if unit == lastUnit || !utf8.ValidString(text) {
				units.Close()
				return result, fmt.Errorf("fragmentos ou texto da unidade inválidos")
			}
			loc, err := parseLocator(locator, text)
			if err != nil {
				units.Close()
				return result, err
			}
			if ordinal < 1 || *loc.Page != ordinal || unit != fmt.Sprintf("page-%06d", ordinal) {
				units.Close()
				return result, fmt.Errorf("localizador não corresponde à unidade")
			}
			lastUnit = unit
			if text == "" && !id.Valid {
				continue
			}
			if text == "" || !id.Valid || !stable.Valid || !start.Valid || !end.Valid || start.Int64 != 0 || end.Int64 != int64(len(text)) || stable.String != fragmentID(d.extraction, d.profile, unit, text) {
				units.Close()
				return result, fmt.Errorf("fragmento ausente ou inconsistente")
			}
			if rebuild {
				if _, err := tx.ExecContext(ctx, `INSERT INTO fragment_fts(rowid,title,body) VALUES(?,?,?)`, id.Int64, d.title, text); err != nil {
					units.Close()
					return result, err
				}
			} else {
				var indexedTitle, indexedText string
				if err := tx.QueryRowContext(ctx, `SELECT title,body FROM fragment_fts WHERE rowid=?`, id.Int64).Scan(&indexedTitle, &indexedText); err != nil {
					units.Close()
					return result, fmt.Errorf("entrada do índice ausente: %w", err)
				}
				if indexedTitle != d.title || indexedText != text {
					units.Close()
					return result, fmt.Errorf("entrada do índice divergente")
				}
			}
			if err := enc.Encode(stable.String); err != nil {
				units.Close()
				return result, err
			}
			count++
		}
		if err := units.Err(); err != nil {
			units.Close()
			return result, err
		}
		if err := units.Close(); err != nil {
			return result, err
		}
		var stored int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fragments WHERE extraction_id=? AND profile_id=?`, d.extraction, d.profile).Scan(&stored); err != nil {
			return result, err
		}
		if count == 0 || count != stored {
			return result, fmt.Errorf("conjunto de fragmentos incompleto ou inconsistente")
		}
		result.Documents++
		result.Fragments += count
	}
	result.Digest = hex.EncodeToString(h.Sum(nil))
	if !rebuild {
		var version, count int
		var config, activeDigest string
		if err := tx.QueryRowContext(ctx, `SELECT version,config_json,active_digest FROM index_state WHERE id=1`).Scan(&version, &config, &activeDigest); err != nil {
			return result, fmt.Errorf("estado do índice ausente: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fragment_fts`).Scan(&count); err != nil {
			return result, err
		}
		if version != 1 || config != indexConfig || activeDigest != result.Digest || count != result.Fragments {
			return result, fmt.Errorf("índice desatualizado ou inconsistente")
		}
		return result, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO index_state(id,version,config_json,active_digest) VALUES(1,1,?,?) ON CONFLICT(id) DO UPDATE SET version=excluded.version,config_json=excluded.config_json,active_digest=excluded.active_digest`, indexConfig, result.Digest); err != nil {
		return result, err
	}
	return result, nil
}
