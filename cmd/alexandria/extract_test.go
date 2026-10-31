package main

import (
	"bytes"
	"testing"
)

func TestExtractCLIUsage(t *testing.T) {
	for _, args := range [][]string{
		{"extract"},
		{"extract", "--catalog", "file"},
		{"extract", "--catalog", "file", "--source", "id", "--all"},
		{"extract", "--catalog", "file", "--all", "--mode", "unknown"},
		{"extract", "--catalog", "file", "--all", "--timeout", "0s"},
		{"extract", "--catalog", "file", "--all", "--max-record-bytes", "0"},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 2 || errOut.Len() == 0 {
			t.Fatalf("%v: code=%d, stderr=%s", args, code, &errOut)
		}
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"extract", "--help"}, &out, &errOut); code != 0 {
		t.Fatalf("help code=%d", code)
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"extract", "--catalog", "../../testdata/catalog/valid.json", "--source", "absent"}, &out, &errOut); code != 2 {
		t.Fatalf("source selection code=%d", code)
	}
}
