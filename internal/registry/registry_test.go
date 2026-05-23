package registry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thesouldev/goboxd/internal/registry"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "langs-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestLoadValid(t *testing.T) {
	yaml := `
languages:
  - id: py3
    name: Python 3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
`
	path := writeYAML(t, yaml)
	reg, err := registry.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("expected 1 language, got %d", reg.Len())
	}
	lang := reg.Get("py3")
	if lang == nil {
		t.Fatal("py3 not found")
	}
	if lang.IsCompiled() {
		t.Error("py3 should not be compiled")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := registry.Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadDuplicateID(t *testing.T) {
	yaml := `
languages:
  - id: py3
    name: Python 3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
  - id: py3
    name: Python 3 duplicate
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for duplicate id")
	}
}

func TestLoadMissingRunCmd(t *testing.T) {
	yaml := `
languages:
  - id: broken
    name: Broken
    source_filename: x.py
    run:
      cmd: ""
      args: []
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for missing run.cmd")
	}
}
