package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestVersion(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"version"}, &out, &err); code != 0 || out.String() != "stoa-inspect 0.3.0\n" {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}

func TestWatchRejectsRemoteAddressByDefault(t *testing.T) {
	var out, err bytes.Buffer
	path := filepath.Join("..", "..", "examples", "valid.ndjson")
	if code := run([]string{"watch", "--listen", "0.0.0.0:8787", path}, &out, &err); code != 2 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}
func TestValidExample(t *testing.T) {
	var out, err bytes.Buffer
	path := filepath.Join("..", "..", "examples", "valid.ndjson")
	if code := run([]string{"audit", path}, &out, &err); code != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}
func TestInvalidExampleReturnsOne(t *testing.T) {
	var out, err bytes.Buffer
	path := filepath.Join("..", "..", "examples", "invalid.ndjson")
	if code := run([]string{"audit", path}, &out, &err); code != 1 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}
