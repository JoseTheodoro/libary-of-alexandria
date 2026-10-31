package extract

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/strictjson"
)

type Options struct {
	Python    string
	Adapter   string
	Output    string
	Mode      string
	Timeout   time.Duration
	MaxRecord int
}

type Manifest struct {
	SchemaVersion int     `json:"schema_version"`
	Key           string  `json:"key"`
	SourceID      string  `json:"source_id"`
	SourceSHA256  string  `json:"source_sha256"`
	AdapterSHA256 string  `json:"adapter_sha256"`
	Engine        Engine  `json:"engine"`
	Mode          string  `json:"mode"`
	EventsSHA256  string  `json:"events_sha256"`
	Summary       Summary `json:"summary"`
}

type Result struct {
	SourceID  string  `json:"source_id"`
	Key       string  `json:"key,omitempty"`
	Directory string  `json:"directory,omitempty"`
	Reused    bool    `json:"reused"`
	Summary   Summary `json:"summary"`
	Error     string  `json:"error,omitempty"`
}

type limitedLog struct {
	data      []byte
	truncated bool
}

func (b *limitedLog) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > 16*1024-len(b.data) {
		b.truncated = true
	}
	b.data = append(b.data, p[:min(len(p), 16*1024-len(b.data))]...)
	return n, nil
}

func validateEngine(e Engine) error {
	if e.AdapterVersion != "1" || e.Library != "pypdf" || e.LibraryVersion != "6.20.1" || e.PythonVersion == "" {
		return fmt.Errorf("adaptador ou versão de pypdf não suportados")
	}
	return nil
}

func engineInfo(ctx context.Context, options Options) (Engine, error) {
	var e Engine
	cmd := exec.CommandContext(ctx, options.Python, options.Adapter, "--info")
	var stdout, stderr limitedLog
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return e, fmt.Errorf("consultar adaptador: %w", err)
	}
	if err := strictjson.Decode(stdout.data, &e); err != nil {
		return e, fmt.Errorf("contrato de versão inválido: %w", err)
	}
	return e, validateEngine(e)
}

func keyFor(source catalog.Source, adapterSHA string, engine Engine, mode string) string {
	data, _ := json.Marshal(struct {
		SchemaVersion                         int
		SourceID, SourceSHA256, AdapterSHA256 string
		Engine                                Engine
		Mode                                  string
	}{SchemaVersion, source.ID, source.SHA256, adapterSHA, engine, mode})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return err
}

func existing(directory string, expected Manifest, maxRecord int) (Summary, error) {
	var manifest Manifest
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return Summary{}, err
	}
	if err = strictjson.Decode(data, &manifest); err != nil {
		return Summary{}, err
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Key != expected.Key || manifest.SourceID != expected.SourceID || manifest.SourceSHA256 != expected.SourceSHA256 || manifest.AdapterSHA256 != expected.AdapterSHA256 || manifest.Engine != expected.Engine || manifest.Mode != expected.Mode {
		return Summary{}, fmt.Errorf("manifesto incompatível")
	}
	path := filepath.Join(directory, "events.jsonl")
	digest, err := catalog.Checksum(path)
	if err != nil {
		return Summary{}, err
	}
	if digest != manifest.EventsSHA256 {
		return Summary{}, fmt.Errorf("artefato corrompido")
	}
	f, err := os.Open(path)
	if err != nil {
		return Summary{}, err
	}
	defer f.Close()
	summary, err := Validate(f, io.Discard, expected.SourceID, expected.SourceSHA256, expected.Mode, expected.Engine, maxRecord)
	if err != nil {
		return summary, err
	}
	if summary != manifest.Summary || summary.Status != "complete" {
		return summary, fmt.Errorf("resultado não está completo; preserve a tentativa e investigue antes de reexecutar")
	}
	return summary, nil
}

// Run validates one authorized source and publishes one immutable extraction.
// A corrupt or partial existing result is never overwritten or reused silently.
func Run(ctx context.Context, source catalog.Source, options Options) (result Result, returnErr error) {
	result.SourceID = source.ID
	defer func() {
		if returnErr != nil {
			result.Error = returnErr.Error()
		}
	}()
	if options.Timeout <= 0 || options.MaxRecord <= 0 || options.Output == "" || options.Python == "" || options.Adapter == "" || (options.Mode != "plain" && options.Mode != "layout") {
		return result, fmt.Errorf("opções de extração inválidas")
	}
	if source.Format != "pdf" {
		return result, fmt.Errorf("formato não implementado: %s", source.Format)
	}
	source.SHA256 = strings.ToLower(source.SHA256)
	if report := catalog.Validate(catalog.Catalog{SchemaVersion: 1, Sources: []catalog.Source{source}}, ".", 1); !report.Valid {
		return result, fmt.Errorf("fonte não passou na validação de catálogo")
	}
	var err error
	source.Path, err = filepath.Abs(source.Path)
	if err != nil {
		return result, err
	}
	options.Adapter, err = filepath.Abs(options.Adapter)
	if err != nil {
		return result, err
	}
	options.Output, err = filepath.Abs(options.Output)
	if err != nil {
		return result, err
	}
	adapterSHA, err := catalog.Checksum(options.Adapter)
	if err != nil {
		return result, err
	}
	infoCtx, cancel := context.WithTimeout(ctx, min(options.Timeout, 30*time.Second))
	engine, err := engineInfo(infoCtx, options)
	cancel()
	if err != nil {
		return result, err
	}
	result.Key = keyFor(source, adapterSHA, engine, options.Mode)
	parent := filepath.Join(options.Output, source.ID)
	result.Directory = filepath.Join(parent, result.Key)
	expected := Manifest{SchemaVersion: SchemaVersion, Key: result.Key, SourceID: source.ID, SourceSHA256: source.SHA256, AdapterSHA256: adapterSHA, Engine: engine, Mode: options.Mode}
	if err = os.MkdirAll(parent, 0700); err != nil {
		return result, err
	}
	lock := result.Directory + ".lock"
	if err = os.Mkdir(lock, 0700); err != nil {
		return result, fmt.Errorf("extração ocupada ou lock existente; verifique a tentativa anterior: %w", err)
	}
	defer os.Remove(lock)
	if _, err = os.Stat(result.Directory); err == nil {
		result.Summary, err = existing(result.Directory, expected, options.MaxRecord)
		if err != nil {
			return result, fmt.Errorf("resultado existente recusado: %w", err)
		}
		result.Reused = true
		return result, nil
	} else if !os.IsNotExist(err) {
		return result, err
	}
	temp, err := os.MkdirTemp(parent, ".extract-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(temp) // Only the temporary directory created by this run.
	path := filepath.Join(temp, "events.jsonl")
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return result, err
	}
	request, _ := json.Marshal(struct {
		SchemaVersion int    `json:"schema_version"`
		SourceID      string `json:"source_id"`
		SourceSHA256  string `json:"source_sha256"`
		Path          string `json:"path"`
		Mode          string `json:"mode"`
	}{SchemaVersion, source.ID, source.SHA256, source.Path, options.Mode})
	runCtx, stop := context.WithTimeout(ctx, options.Timeout)
	defer stop()
	cmd := exec.CommandContext(runCtx, options.Python, options.Adapter)
	cmd.Stdin = bytes.NewReader(request)
	cmd.WaitDelay = time.Second
	var stderr limitedLog
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		out.Close()
		return result, err
	}
	if err = cmd.Start(); err != nil {
		out.Close()
		return result, err
	}
	summary, validationErr := Validate(stdout, out, source.ID, source.SHA256, options.Mode, engine, options.MaxRecord)
	if validationErr != nil {
		stop()
	}
	processErr := cmd.Wait()
	if err = out.Sync(); err == nil {
		err = out.Close()
	} else {
		out.Close()
	}
	// Failures retain bounded diagnostics in a private, explicitly failed artifact.
	if validationErr != nil || processErr != nil || err != nil {
		failure := struct {
			SchemaVersion       int    `json:"schema_version"`
			Status              string `json:"status"`
			SourceID            string `json:"source_id"`
			Cause               string `json:"cause"`
			Diagnostic          string `json:"diagnostic"`
			DiagnosticTruncated bool   `json:"diagnostic_truncated"`
		}{1, "failed", source.ID, errors.Join(validationErr, processErr, err).Error(), string(stderr.data), stderr.truncated}
		if writeErr := writeJSON(filepath.Join(temp, "failure.json"), failure); writeErr != nil {
			return result, writeErr
		}
		// Every failed attempt retains its own diagnostics; a later failure must
		// not discard its log merely because an earlier failed attempt exists.
		failedDir := result.Directory + ".failed-" + filepath.Base(temp)
		if renameErr := os.Rename(temp, failedDir); renameErr != nil {
			return result, renameErr
		}
		if validationErr != nil {
			return result, validationErr
		}
		if processErr != nil {
			return result, fmt.Errorf("processo de extração falhou: %w", processErr)
		}
		return result, err
	}
	actual, err := catalog.Checksum(source.Path)
	if err != nil {
		return result, err
	}
	if actual != source.SHA256 {
		return result, fmt.Errorf("a fonte mudou durante a extração")
	}
	// The adapter itself must also remain stable throughout the run.
	currentAdapterSHA, err := catalog.Checksum(options.Adapter)
	if err != nil || currentAdapterSHA != adapterSHA {
		return result, fmt.Errorf("o adaptador mudou durante a extração")
	}
	expected.EventsSHA256, err = catalog.Checksum(path)
	if err != nil {
		return result, err
	}
	expected.Summary = summary
	if err = writeJSON(filepath.Join(temp, "manifest.json"), expected); err != nil {
		return result, err
	}
	if err = os.Rename(temp, result.Directory); err != nil {
		return result, err
	}
	result.Summary = summary
	if summary.Status != "complete" {
		return result, fmt.Errorf("extração parcial; consulte os diagnósticos privados")
	}
	return result, nil
}
