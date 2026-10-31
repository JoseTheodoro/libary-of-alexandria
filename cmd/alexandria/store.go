package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/store"
)

func runStore(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "init" {
		fmt.Fprintln(stderr, "Uso: alexandria store init --db <arquivo.sqlite>")
		return 2
	}
	flags := flag.NewFlagSet("store init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("db", "", "arquivo SQLite local")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *path == "" || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	ctx := context.Background()
	s, err := store.Init(ctx, *path)
	if err != nil {
		fmt.Fprintln(stderr, "Erro ao inicializar banco:", err)
		return 1
	}
	defer s.Close()
	version, err := s.SQLiteVersion(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "Erro ao consultar versão SQLite:", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(struct {
		SchemaVersion int    `json:"schema_version"`
		SQLiteVersion string `json:"sqlite_version"`
	}{1, version}); err != nil {
		fmt.Fprintln(stderr, "Erro ao escrever resultado:", err)
		return 1
	}
	return 0
}
