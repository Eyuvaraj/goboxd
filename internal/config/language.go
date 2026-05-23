package config

// LimitsDef holds resource limits for a build or run phase.
type LimitsDef struct {
	WallTimeS   int `yaml:"wall_time_s"`
	MemoryKB    int `yaml:"memory_kb"`
	MaxProcesses int `yaml:"max_processes"`
}

// PhaseDef describes either the build or run phase of a language.
type PhaseDef struct {
	Cmd          string    `yaml:"cmd"`
	Args         []string  `yaml:"args"`
	Limits       LimitsDef `yaml:"limits"`
	FlagAllowlist []string `yaml:"flag_allowlist"`
}

// LanguageDef is the YAML schema for a single language entry.
type LanguageDef struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`

	// SourceFilename is used when the filename is fixed (e.g. "solution.py").
	// If SourceFilenameStrategy is "from_request", the client supplies it.
	SourceFilename         string `yaml:"source_filename"`
	SourceFilenameStrategy string `yaml:"source_filename_strategy"` // "" or "from_request"

	// ArtifactFilename is the compiled output name.
	// If ArtifactFilenameStrategy is "from_request", the client supplies it.
	ArtifactFilename         string `yaml:"artifact_filename"`
	ArtifactFilenameStrategy string `yaml:"artifact_filename_strategy"` // "" or "from_request"

	Build *PhaseDef `yaml:"build"` // nil for interpreted languages
	Run   PhaseDef  `yaml:"run"`
}

// IsCompiled returns true when the language has a build phase.
func (l *LanguageDef) IsCompiled() bool {
	return l.Build != nil
}

// LanguagesFile is the top-level YAML structure.
type LanguagesFile struct {
	Languages []LanguageDef `yaml:"languages"`
}
