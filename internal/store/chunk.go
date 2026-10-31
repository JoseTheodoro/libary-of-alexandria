package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// The baseline deliberately keeps a whole unit when safe structural boundaries
// cannot be established; the target is advisory, not a truncation limit.
const chunkProfile = `{"version":1,"strategy":"whole_unit","target_bytes":4096,"overlap_bytes":0}`

type ChunkResult struct {
	ExtractionID string `json:"extraction_id"`
	ProfileID    string `json:"profile_id"`
	Fragments    int    `json:"fragments"`
	Oversized    int    `json:"oversized"`
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Chunk stores exact byte ranges into canonical units without activating them.
// Repetition verifies the existing projection instead of silently replacing it.
func (s *Store) Chunk(ctx context.Context, extractionID string) (ChunkResult, error) {
	result := ChunkResult{ExtractionID: extractionID, ProfileID: digest([]byte(chunkProfile))}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM extraction_revisions WHERE id=?`, extractionID).Scan(&status); err != nil {
		return result, fmt.Errorf("extração não encontrada: %w", err)
	}
	if status != "complete" {
		return result, fmt.Errorf("extração incompleta não pode gerar fragmentos")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chunk_profiles(id,config_json) VALUES(?,?) ON CONFLICT(id) DO NOTHING`, result.ProfileID, chunkProfile); err != nil {
		return result, err
	}
	var config string
	if err := tx.QueryRowContext(ctx, `SELECT config_json FROM chunk_profiles WHERE id=?`, result.ProfileID).Scan(&config); err != nil {
		return result, err
	}
	if config != chunkProfile {
		return result, fmt.Errorf("perfil de fragmentação divergente")
	}
	rows, err := tx.QueryContext(ctx, `SELECT unit_id,text FROM document_units WHERE extraction_id=? ORDER BY ordinal`, extractionID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var unit, text string
		if err := rows.Scan(&unit, &text); err != nil {
			return result, err
		}
		if !utf8.ValidString(text) {
			return result, fmt.Errorf("unidade %q contém UTF-8 inválido", unit)
		}
		if len(text) == 0 {
			continue
		}
		identity, err := json.Marshal(struct {
			Extraction string `json:"extraction"`
			Profile    string `json:"profile"`
			Unit       string `json:"unit"`
			Start      int    `json:"start"`
			End        int    `json:"end"`
			TextSHA256 string `json:"text_sha256"`
		}{extractionID, result.ProfileID, unit, 0, len(text), digest([]byte(text))})
		if err != nil {
			return result, err
		}
		stableID := digest(identity)
		if _, err := tx.ExecContext(ctx, `INSERT INTO fragments(stable_id,extraction_id,unit_id,profile_id,start_byte,end_byte) VALUES(?,?,?,?,?,?) ON CONFLICT DO NOTHING`, stableID, extractionID, unit, result.ProfileID, 0, len(text)); err != nil {
			return result, err
		}
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT stable_id FROM fragments WHERE extraction_id=? AND unit_id=? AND profile_id=? AND start_byte=0 AND end_byte=?`, extractionID, unit, result.ProfileID, len(text)).Scan(&actual); err != nil {
			return result, fmt.Errorf("fragmento conflitante: %w", err)
		}
		if actual != stableID {
			return result, fmt.Errorf("fragmento existente divergente")
		}
		result.Fragments++
		if len(text) > 4096 {
			result.Oversized++
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fragments WHERE extraction_id=? AND profile_id=?`, extractionID, result.ProfileID).Scan(&count); err != nil {
		return result, err
	}
	if count != result.Fragments {
		return result, fmt.Errorf("fragmentos existentes divergentes")
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
