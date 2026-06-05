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

func TestLoadUnknownPlaceholder(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{srce}}"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for unknown placeholder {{srce}}")
	}
}

func TestLoadUnclosedPlaceholder(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for unclosed {{")
	}
}

func TestLoadZeroWallTime(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 0, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for wall_time_s: 0")
	}
}

func TestLoadZeroMemory(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 5, memory_kb: 0, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for memory_kb: 0")
	}
}

func TestLoadInvalidSourceFilenameStrategy(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    source_filename_strategy: typo
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for invalid source_filename_strategy")
	}
}

func TestLoadInvalidBindMount(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    source_filename: solution.py
    bind_mounts:
      - relative/path
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for relative bind_mounts path")
	}
}

func TestLoadRootBindMount(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    source_filename: solution.py
    bind_mounts:
      - /
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for bind_mounts entry /")
	}
}

func TestLoadValidBindMount(t *testing.T) {
	yaml := `
languages:
  - id: swift
    name: Swift
    source_filename: solution.swift
    bind_mounts:
      - /opt/swift
    run:
      cmd: /opt/swift/bin/swift
      args: ["{{source}}"]
      limits: {wall_time_s: 10, memory_kb: 262144, max_processes: 64}
`
	path := writeYAML(t, yaml)
	reg, err := registry.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lang := reg.Get("swift")
	if lang == nil {
		t.Fatal("swift not found")
	}
	if len(lang.BindMounts) != 1 || lang.BindMounts[0] != "/opt/swift" {
		t.Errorf("bind_mounts not preserved: %v", lang.BindMounts)
	}
}

func TestLoadMissingSourceFilename(t *testing.T) {
	yaml := `
languages:
  - id: bad
    name: Bad
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: {wall_time_s: 5, memory_kb: 1024, max_processes: 10}
`
	path := writeYAML(t, yaml)
	_, err := registry.Load(path)
	if err == nil {
		t.Error("expected error for missing source_filename with no strategy")
	}
}
