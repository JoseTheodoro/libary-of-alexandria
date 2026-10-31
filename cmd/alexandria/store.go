package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/store"
)

func runStore(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "import" {
		return runStoreImport(args[1:], stdout, stderr)
	}
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
	}{store.SchemaVersion, version}); err != nil {
		fmt.Fprintln(stderr, "Erro ao escrever resultado:", err)
		return 1
	}
	return 0
}

func runStoreImport(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("store import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "", "banco SQLite existente")
	manifestPath := flags.String("catalog", "", "catálogo JSON local")
	sourceID := flags.String("source", "", "ID da fonte")
	artifact := flags.String("artifact", "", "diretório da extração validada")
	maxRecord := flags.Int("max-record-bytes", 16*1024*1024, "limite por registro JSONL")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *dbPath == "" || *manifestPath == "" || *sourceID == "" || *artifact == "" || *maxRecord <= 0 || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	info, err := os.Stat(*dbPath)
	if err != nil || !info.Mode().IsRegular() {
		fmt.Fprintln(stderr, "Banco existente não encontrado; execute store init primeiro")
		return 1
	}
	path, err := filepath.Abs(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c, err := catalog.Decode(f)
	f.Close()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if c.SchemaVersion != catalog.SchemaVersion {
		fmt.Fprintln(stderr, "Versão de catálogo incompatível")
		return 1
	}
	var selected []catalog.Source
	for _, source := range c.Sources {
		if source.ID == *sourceID {
			if !filepath.IsAbs(source.Path) {
				source.Path = filepath.Join(filepath.Dir(path), source.Path)
			}
			selected = append(selected, source)
		}
	}
	if len(selected) != 1 {
		fmt.Fprintln(stderr, "ID ausente ou duplicado no catálogo")
		return 1
	}
	ctx := context.Background()
	s, err := store.Init(ctx, *dbPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer s.Close()
	result, err := s.Import(ctx, selected[0], *artifact, *maxRecord)
	if err != nil {
		fmt.Fprintln(stderr, "Erro ao importar:", err)
		return 1
	}
	if err = json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
