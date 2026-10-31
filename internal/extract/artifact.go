package extract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JoseTheodoro/libary-of-alexandria/internal/catalog"
	"github.com/JoseTheodoro/libary-of-alexandria/internal/strictjson"
)

// OpenVerified validates a complete extraction against its source and returns
// an open stream rewound to its start. The caller must close it and recheck its
// digest while consuming it, before committing any imported data.
func OpenVerified(source catalog.Source, directory string, maxRecord int) (Manifest, *os.File, error) {
	var manifest Manifest
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return manifest, nil, err
	}
	if err := strictjson.Decode(data, &manifest); err != nil {
		return manifest, nil, fmt.Errorf("manifesto inválido: %w", err)
	}
	source.SHA256 = strings.ToLower(source.SHA256)
	if manifest.SchemaVersion != SchemaVersion || manifest.SourceID != source.ID || manifest.SourceSHA256 != source.SHA256 || (manifest.Mode != "plain" && manifest.Mode != "layout") || manifest.Summary.Status != "complete" {
		return manifest, nil, fmt.Errorf("manifesto incompleto ou incompatível com a fonte")
	}
	if err := validateEngine(manifest.Engine); err != nil {
		return manifest, nil, err
	}
	for _, digest := range []string{manifest.AdapterSHA256, manifest.EventsSHA256} {
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != sha256.Size || digest != strings.ToLower(digest) {
			return manifest, nil, fmt.Errorf("digest inválido no manifesto")
		}
	}
	if manifest.Key != keyFor(source, manifest.AdapterSHA256, manifest.Engine, manifest.Mode) {
		return manifest, nil, fmt.Errorf("identidade da extração incompatível")
	}
	f, err := os.Open(filepath.Join(directory, "events.jsonl"))
	if err != nil {
		return manifest, nil, err
	}
	h := sha256.New()
	summary, err := Validate(io.TeeReader(f, h), io.Discard, source.ID, source.SHA256, manifest.Mode, manifest.Engine, maxRecord)
	if err == nil && (summary != manifest.Summary || hex.EncodeToString(h.Sum(nil)) != manifest.EventsSHA256) {
		err = fmt.Errorf("eventos ou contagens divergem do manifesto")
	}
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		f.Close()
		return manifest, nil, err
	}
	return manifest, f, nil
}
