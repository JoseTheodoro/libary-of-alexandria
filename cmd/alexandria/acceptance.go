package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/store"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/strictjson"
)

func runAcceptance(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("acceptance", flag.ContinueOnError)
	flags.SetOutput(stderr)
	db := flags.String("db", "", "banco SQLite existente")
	path := flags.String("suite", "", "gabarito JSON local")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *db == "" || *path == "" || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	f, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	const maxSuiteBytes = 4 << 20
	data, err := io.ReadAll(io.LimitReader(f, maxSuiteBytes+1))
	closeErr := f.Close()
	if err != nil || closeErr != nil || len(data) > maxSuiteBytes {
		fmt.Fprintln(stderr, "Erro ao ler gabarito ou tamanho excedido")
		return 1
	}
	var suite store.AcceptanceSuite
	if err := strictjson.Decode(data, &suite); err != nil {
		fmt.Fprintln(stderr, "Gabarito inválido:", err)
		return 1
	}
	var fields struct {
		Cases []map[string]json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		fmt.Fprintln(stderr, "Gabarito inválido:", err)
		return 1
	}
	for _, c := range fields.Cases {
		for _, name := range []string{"start_byte", "end_byte"} {
			if v, ok := c[name]; !ok || string(v) == "null" {
				fmt.Fprintln(stderr, "Gabarito inválido: intervalo incompleto")
				return 1
			}
		}
	}
	ctx := context.Background()
	s, err := store.OpenReadOnly(ctx, *db)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer s.Close()
	report, err := s.EvaluateAcceptance(ctx, suite)
	if err != nil {
		fmt.Fprintln(stderr, "Erro na avaliação:", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !report.Passed {
		return 1
	}
	return 0
}
