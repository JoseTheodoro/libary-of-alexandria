package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
)

const usage = `Uso:
  alexandria catalog validate [--require-count N] <catalog.json>
  alexandria catalog checksum <arquivo>
  alexandria extract --catalog <catálogo> (--source <id> | --all) [opções]

Validação local, sem alterar catálogo ou fontes. O aceite de M1 exige --require-count 9.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "extract" {
		return runExtract(args[1:], stdout, stderr)
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) < 2 || args[0] != "catalog" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[1] {
	case "checksum":
		if len(args) != 3 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		hash, err := catalog.Checksum(args[2])
		if err != nil {
			fmt.Fprintln(stderr, "Erro ao calcular SHA-256:", err)
			return 1
		}
		if _, err := fmt.Fprintln(stdout, hash); err != nil {
			fmt.Fprintln(stderr, "Erro ao escrever resultado:", err)
			return 1
		}
		return 0
	case "validate":
		flags := flag.NewFlagSet("catalog validate", flag.ContinueOnError)
		flags.SetOutput(stderr)
		count := flags.Int("require-count", 0, "quantidade exata de fontes exigida (0: sem quantidade fixa)")
		flags.Usage = func() { fmt.Fprint(stderr, usage); flags.PrintDefaults() }
		if err := flags.Parse(args[2:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if flags.NArg() != 1 || *count < 0 {
			flags.Usage()
			return 2
		}
		path, err := filepath.Abs(flags.Arg(0))
		if err != nil {
			fmt.Fprintln(stderr, "Erro no caminho do catálogo:", err)
			return 1
		}
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(stderr, "Erro ao abrir catálogo:", err)
			return 1
		}
		defer f.Close()
		c, err := catalog.Decode(f)
		if err != nil {
			fmt.Fprintln(stderr, "Erro:", err)
			return 1
		}
		report := catalog.Validate(c, filepath.Dir(path), *count)
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(stderr, "Erro ao escrever relatório:", err)
			return 1
		}
		if !report.Valid {
			return 1
		}
		return 0
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}
