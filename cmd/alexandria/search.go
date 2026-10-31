package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/store"
)

func runSearch(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("search", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "", "banco SQLite existente")
	query := flags.String("query", "", "consulta lexical")
	limit := flags.Int("limit", 5, "máximo de resultados (1 a 100)")
	source := flags.String("source", "", "filtrar por ID da fonte")
	jsonOutput := flags.Bool("json", false, "retornar JSON")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *dbPath == "" || strings.TrimSpace(*query) == "" || !utf8.ValidString(*query) || strings.ContainsRune(*query, '\x00') || *limit < 1 || *limit > 100 || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	ctx := context.Background()
	s, err := store.OpenReadOnly(ctx, *dbPath)
	if err != nil {
		fmt.Fprintln(stderr, "Erro ao abrir banco:", err)
		return 1
	}
	defer s.Close()
	hits, err := s.Search(ctx, *query, *source, *limit)
	if err != nil {
		fmt.Fprintln(stderr, "Erro na busca:", err)
		return 1
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(hits); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if len(hits) == 0 {
		if _, err := fmt.Fprintln(stdout, "Nenhum resultado."); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	for _, hit := range hits {
		edition := "não informada"
		if hit.Edition != nil {
			edition = *hit.Edition
		}
		if _, err := fmt.Fprintf(stdout, "%s — %s | Edição: %s\nFonte: %s | SHA-256: %s\nExtração: %s | Unidade: %s | Fragmento: %s\nPágina: %d | Bytes do texto extraído: %d–%d\n%s\n", hit.Title, strings.Join(hit.Authors, ", "), edition, hit.SourceID, hit.SourceSHA256, hit.ExtractionID, hit.UnitID, hit.FragmentID, *hit.Locator.Page, hit.StartByte, hit.EndByte, hit.Text); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, d := range hit.Diagnostics {
			if _, err := fmt.Fprintf(stdout, "Diagnóstico [%s/%s]: %s\n", d.Severity, d.Code, d.Message); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
	}
	return 0
}
