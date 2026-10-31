package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// AcceptanceSuite is a private, pre-reviewed gold set. Offsets refer to the
// exact UTF-8 unit text, not to bytes in the original PDF.
type AcceptanceSuite struct {
	SchemaVersion int              `json:"schema_version"`
	IndexDigest   string           `json:"index_digest"`
	Cases         []AcceptanceCase `json:"cases"`
}

type AcceptanceCase struct {
	ID           string `json:"id"`
	Query        string `json:"query"`
	SourceID     string `json:"source_id"`
	SourceSHA256 string `json:"source_sha256"`
	ExtractionID string `json:"extraction_id"`
	UnitID       string `json:"unit_id"`
	StartByte    int    `json:"start_byte"`
	EndByte      int    `json:"end_byte"`
}

type AcceptanceCaseResult struct {
	ID          string   `json:"id"`
	Rank        int      `json:"rank"` // zero means no expected reference in the top five.
	FragmentIDs []string `json:"fragment_ids"`
}

type AcceptanceReport struct {
	SchemaVersion     int                    `json:"schema_version"`
	SuiteSHA256       string                 `json:"suite_sha256"`
	IndexDigest       string                 `json:"index_digest"`
	SQLiteVersion     string                 `json:"sqlite_version"`
	Total             int                    `json:"total"`
	Hits              int                    `json:"hits_at_5"`
	ReferencesChecked int                    `json:"references_checked"`
	Passed            bool                   `json:"passed"`
	Cases             []AcceptanceCaseResult `json:"cases"`
	Audit             *QualityAuditSummary   `json:"quality_audit,omitempty"`
}

// QualityAudit is the private #12 sidecar, tied to extraction artifacts.
type QualityAudit struct {
	SchemaVersion    int             `json:"schema_version"`
	Method           string          `json:"method"`
	Samples          []QualitySample `json:"samples"`
	Diagnostics      map[string]int  `json:"diagnostics"`
	KnownLimitations []string        `json:"known_limitations"`
}
type QualitySample struct {
	SourceID             string  `json:"source_id"`
	SourceSHA256         string  `json:"source_sha256"`
	ExtractionKey        string  `json:"extraction_key"`
	EventsSHA256         string  `json:"events_sha256"`
	Page                 int     `json:"page"`
	TokenOrderSimilarity float64 `json:"token_order_similarity"`
	Warning              *string `json:"warning"`
}
type QualityAuditSummary struct {
	SHA256           string           `json:"sha256"`
	Method           string           `json:"method"`
	Samples          int              `json:"samples"`
	Warnings         int              `json:"warnings"`
	SampleWarnings   []QualityWarning `json:"sample_warnings"`
	Diagnostics      map[string]int   `json:"diagnostics"`
	KnownLimitations []string         `json:"known_limitations"`
}

type QualityWarning struct {
	SourceID string `json:"source_id"`
	Page     int    `json:"page"`
	Message  string `json:"message"`
}

// EvaluateAcceptance enforces the M1 gate of nine sources, two queries each,
// and at least fifteen expected references in the global top five.
func (s *Store) EvaluateAcceptance(ctx context.Context, suite AcceptanceSuite) (AcceptanceReport, error) {
	return s.evaluate(ctx, suite, 9, 2, 15)
}

// EvaluateAcceptanceWithAudit also validates and records the #12 audit sidecar.
func (s *Store) EvaluateAcceptanceWithAudit(ctx context.Context, suite AcceptanceSuite, audit QualityAudit) (AcceptanceReport, error) {
	return s.evaluateWithAudit(ctx, suite, 9, 2, 15, &audit)
}

func (s *Store) evaluate(ctx context.Context, suite AcceptanceSuite, sourceCount, perSource, minimum int) (AcceptanceReport, error) {
	return s.evaluateWithAudit(ctx, suite, sourceCount, perSource, minimum, nil)
}

func (s *Store) evaluateWithAudit(ctx context.Context, suite AcceptanceSuite, sourceCount, perSource, minimum int, audit *QualityAudit) (AcceptanceReport, error) {
	report := AcceptanceReport{SchemaVersion: 1, Total: len(suite.Cases), Cases: []AcceptanceCaseResult{}}
	if suite.SchemaVersion != 1 || len(suite.Cases) != sourceCount*perSource || strings.TrimSpace(suite.IndexDigest) == "" {
		return report, fmt.Errorf("suíte de aceite incompleta ou versão inválida")
	}
	encoded, err := json.Marshal(suite)
	if err != nil {
		return report, err
	}
	report.SuiteSHA256 = digest(encoded)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	index, err := inspectIndex(ctx, tx, false)
	if err != nil {
		return report, fmt.Errorf("índice inválido: %w", err)
	}
	if index.Digest != suite.IndexDigest || index.Documents != sourceCount {
		return report, fmt.Errorf("índice ou quantidade de fontes diverge da suíte")
	}
	report.IndexDigest = index.Digest
	if err := tx.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&report.SQLiteVersion); err != nil {
		return report, err
	}
	if audit != nil {
		summary, err := validateQualityAudit(ctx, tx, *audit, sourceCount)
		if err != nil {
			return report, err
		}
		report.Audit = &summary
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	// Validate every expected span before running the first query.
	for _, c := range suite.Cases {
		if strings.TrimSpace(c.ID) == "" || seen[c.ID] || strings.TrimSpace(c.Query) == "" || !utf8.ValidString(c.Query) || strings.ContainsRune(c.Query, '\x00') || c.SourceID == "" || c.ExtractionID == "" || c.UnitID == "" {
			return report, fmt.Errorf("caso inválido ou ID duplicado")
		}
		seen[c.ID] = true
		counts[c.SourceID]++
		var text, sha string
		err := tx.QueryRowContext(ctx, `SELECT u.text,e.source_sha256 FROM active_documents a JOIN extraction_revisions e ON e.id=a.extraction_id AND e.source_id=a.source_id JOIN document_units u ON u.extraction_id=e.id WHERE a.source_id=? AND a.extraction_id=? AND u.unit_id=?`, c.SourceID, c.ExtractionID, c.UnitID).Scan(&text, &sha)
		if err != nil {
			return report, fmt.Errorf("caso %s: referência não está ativa: %w", c.ID, err)
		}
		if sha != c.SourceSHA256 || c.StartByte < 0 || c.EndByte > len(text) || c.EndByte <= c.StartByte || !utf8.ValidString(text[:c.StartByte]) || !utf8.ValidString(text[:c.EndByte]) || strings.TrimSpace(text[c.StartByte:c.EndByte]) == "" {
			return report, fmt.Errorf("caso %s: revisão ou intervalo inválido", c.ID)
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fragments f JOIN active_documents a ON a.extraction_id=f.extraction_id AND a.profile_id=f.profile_id WHERE a.source_id=? AND f.extraction_id=? AND f.unit_id=? AND f.start_byte<=? AND f.end_byte>=?`, c.SourceID, c.ExtractionID, c.UnitID, c.StartByte, c.EndByte).Scan(&count); err != nil {
			return report, err
		}
		if count == 0 {
			return report, fmt.Errorf("caso %s: nenhum fragmento contém a evidência", c.ID)
		}
	}
	if len(counts) != sourceCount {
		return report, fmt.Errorf("quantidade de fontes no gabarito inválida")
	}
	for _, count := range counts {
		if count != perSource {
			return report, fmt.Errorf("quantidade de consultas por fonte inválida")
		}
	}
	for _, c := range suite.Cases {
		hits, err := searchTx(ctx, tx, c.Query, "", 5)
		if err != nil {
			return report, fmt.Errorf("caso %s: %w", c.ID, err)
		}
		result := AcceptanceCaseResult{ID: c.ID, FragmentIDs: []string{}}
		for i, hit := range hits {
			result.FragmentIDs = append(result.FragmentIDs, hit.FragmentID)
			report.ReferencesChecked++
			if result.Rank == 0 && hit.SourceID == c.SourceID && hit.SourceSHA256 == c.SourceSHA256 && hit.ExtractionID == c.ExtractionID && hit.UnitID == c.UnitID && hit.StartByte <= c.StartByte && hit.EndByte >= c.EndByte {
				result.Rank = i + 1
			}
		}
		if result.Rank > 0 {
			report.Hits++
		}
		report.Cases = append(report.Cases, result)
	}
	report.Passed = report.Hits >= minimum
	return report, nil
}
