// Package strictjson rejects ambiguous JSON and invalid Unicode without repair.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Decode(data []byte, target any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON deve usar UTF-8 válido")
	}
	if err := validateUnicodeEscapes(data); err != nil {
		return err
	}
	if err := uniqueKeys(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return fmt.Errorf("JSON inválido ou ambíguo: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("esperado somente um objeto JSON")
	}
	return nil
}

// encoding/json replaces unpaired UTF-16 surrogates with U+FFFD. Reject them
// before decoding so bibliographic metadata cannot be silently altered.
func validateUnicodeEscapes(data []byte) error {
	inString := false
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || data[i] != '\\' {
			continue
		}
		i++ // An escaped quote or backslash must not affect string boundaries.
		if i >= len(data) || data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return fmt.Errorf("escape Unicode incompleto no catálogo")
		}
		code, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			return fmt.Errorf("escape Unicode inválido no catálogo")
		}
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return fmt.Errorf("escape Unicode com surrogate isolado no catálogo")
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
			return fmt.Errorf("escape Unicode com surrogate sem par no catálogo")
		}
		low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return fmt.Errorf("par de surrogates Unicode inválido no catálogo")
		}
		i += 6
	}
	return nil
}

// encoding/json matches struct fields case-insensitively. Reject repeated keys
// with that same rule, including differently cased permission fields.
func uniqueKeys(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	for d.More() {
		if delim == '{' {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key := strings.ToLower(token.(string))
			if seen[key] {
				return fmt.Errorf("campo repetido %q", key)
			}
			seen[key] = true
		}
		if err := uniqueKeys(d); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
