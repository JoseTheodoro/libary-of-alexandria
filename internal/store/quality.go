package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func validateQualityAudit(ctx context.Context, tx *sql.Tx, audit QualityAudit, sourceCount int) (QualityAuditSummary, error) {
	var summary QualityAuditSummary
	if audit.SchemaVersion != 1 || strings.TrimSpace(audit.Method) == "" || len(audit.Samples) == 0 || audit.Diagnostics == nil || audit.KnownLimitations == nil {
		return summary, fmt.Errorf("auditoria de qualidade incompleta ou incompatível")
	}
	encoded, err := json.Marshal(audit)
	if err != nil {
		return summary, err
	}
	summary = QualityAuditSummary{SHA256: digest(encoded), Method: audit.Method, Samples: len(audit.Samples), Diagnostics: audit.Diagnostics, KnownLimitations: audit.KnownLimitations, SampleWarnings: []QualityWarning{}}
	for code, count := range audit.Diagnostics {
		if strings.TrimSpace(code) == "" || count < 0 {
			return summary, fmt.Errorf("diagnóstico de auditoria inválido")
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT d.code,count(*) FROM diagnostics d JOIN active_documents a ON a.extraction_id=d.extraction_id GROUP BY d.code`)
	if err != nil {
		return summary, err
	}
	actual := map[string]int{}
	for rows.Next() {
		var code string
		var count int
		if err := rows.Scan(&code, &count); err != nil {
			rows.Close()
			return summary, err
		}
		actual[code] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return summary, err
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}
	if len(actual) != len(audit.Diagnostics) {
		return summary, fmt.Errorf("diagnósticos da auditoria divergem das extrações ativas")
	}
	for code, count := range actual {
		if audit.Diagnostics[code] != count {
			return summary, fmt.Errorf("diagnósticos da auditoria divergem das extrações ativas")
		}
	}
	for _, limitation := range audit.KnownLimitations {
		if strings.TrimSpace(limitation) == "" {
			return summary, fmt.Errorf("limitação de auditoria vazia")
		}
	}
	seen := map[string]bool{}
	for _, sample := range audit.Samples {
		if sample.Page < 1 || sample.TokenOrderSimilarity < 0 || sample.TokenOrderSimilarity > 1 || (sample.Warning != nil && strings.TrimSpace(*sample.Warning) == "") {
			return summary, fmt.Errorf("amostra de auditoria inválida")
		}
		var sha, events string
		err := tx.QueryRowContext(ctx, `SELECT e.source_sha256,e.events_sha256 FROM active_documents a JOIN extraction_revisions e ON e.id=a.extraction_id AND e.source_id=a.source_id JOIN document_units u ON u.extraction_id=e.id AND u.ordinal=? WHERE a.source_id=? AND a.extraction_id=?`, sample.Page, sample.SourceID, sample.ExtractionKey).Scan(&sha, &events)
		if err != nil || sha != sample.SourceSHA256 || events != sample.EventsSHA256 {
			return summary, fmt.Errorf("amostra de auditoria não corresponde à extração ativa")
		}
		seen[sample.SourceID] = true
		if sample.Warning != nil {
			summary.Warnings++
			summary.SampleWarnings = append(summary.SampleWarnings, QualityWarning{SourceID: sample.SourceID, Page: sample.Page, Message: *sample.Warning})
		}
	}
	if len(seen) != sourceCount {
		return summary, fmt.Errorf("auditoria não cobre todas as fontes ativas")
	}
	return summary, nil
}
