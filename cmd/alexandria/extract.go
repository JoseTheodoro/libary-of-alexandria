package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/extract"
)

func runExtract(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("extract", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("catalog", "", "catálogo JSON local")
	id := flags.String("source", "", "ID de uma fonte")
	all := flags.Bool("all", false, "extrair todas as fontes")
	output := flags.String("output", "extracted", "diretório privado de artefatos")
	python := flags.String("python", "python3", "executável Python")
	adapter := flags.String("adapter", "adapters/pdf/extract.py", "adaptador PDF local")
	mode := flags.String("mode", "layout", "layout ou plain; configuração imutável da execução")
	timeout := flags.Duration("timeout", 10*time.Minute, "limite por documento")
	maxRecord := flags.Int("max-record-bytes", 16*1024*1024, "limite de bytes por registro JSONL")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *manifestPath == "" || (*all == (*id != "")) || *output == "" || *python == "" || *adapter == "" || *timeout <= 0 || *maxRecord <= 0 || (*mode != "plain" && *mode != "layout") {
		fmt.Fprintln(stderr, "Uso: alexandria extract --catalog <catálogo> (--source <id> | --all) [--output extracted]")
		return 2
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
	selected := []catalog.Source{}
	for _, s := range c.Sources {
		if *all || s.ID == *id {
			if !filepath.IsAbs(s.Path) {
				s.Path = filepath.Join(filepath.Dir(path), s.Path)
			}
			selected = append(selected, s)
		}
	}
	if len(selected) == 0 {
		fmt.Fprintln(stderr, "Nenhuma fonte selecionada")
		return 2
	}
	c.Sources = selected
	if report := catalog.Validate(c, ".", 0); !report.Valid {
		json.NewEncoder(stdout).Encode(report)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	results := []extract.Result{}
	code := 0
	for _, source := range selected {
		result, err := extract.Run(ctx, source, extract.Options{Python: *python, Adapter: *adapter, Output: *output, Mode: *mode, Timeout: *timeout, MaxRecord: *maxRecord})
		results = append(results, result)
		if err != nil {
			code = 1
		}
		if ctx.Err() != nil {
			break
		}
	}
	report := struct {
		SchemaVersion int              `json:"schema_version"`
		Results       []extract.Result `json:"results"`
	}{1, results}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return code
}
