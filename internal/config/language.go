package config

type LimitsDef struct {
	WallTimeS    int `yaml:"wall_time_s"   json:"wall_time_s"   example:"5"`
	MemoryKB     int `yaml:"memory_kb"     json:"memory_kb"     example:"262144"`
	MaxProcesses int `yaml:"max_processes" json:"max_processes" example:"64"`
}

type PhaseDef struct {
	Cmd           string    `yaml:"cmd"`
	Args          []string  `yaml:"args"`
	Limits        LimitsDef `yaml:"limits"`
	FlagAllowlist []string  `yaml:"flag_allowlist"`
}

type LanguageDef struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`

	// SourceFilenameStrategy: "" means use SourceFilename; "from_request" means the client supplies it.
	SourceFilename         string `yaml:"source_filename"`
	SourceFilenameStrategy string `yaml:"source_filename_strategy"`

	// ArtifactFilenameStrategy: "" means use ArtifactFilename; "from_request" means the client supplies it.
	ArtifactFilename         string `yaml:"artifact_filename"`
	ArtifactFilenameStrategy string `yaml:"artifact_filename_strategy"`

	// ProbeArgs overrides the arguments used for the readiness probe (default: ["--version"]).
	// Set this for runtimes that use a different flag, e.g. lua5.4 uses ["-v"].
	ProbeArgs []string `yaml:"probe_args"`

	Env   []string  `yaml:"env"`   // extra KEY=VALUE vars injected into every nsjail invocation
	Build *PhaseDef `yaml:"build"` // nil for interpreted languages
	Run   PhaseDef  `yaml:"run"`
}

// IsCompiled returns true when the language has a build phase.
func (l *LanguageDef) IsCompiled() bool {
	return l.Build != nil
}

type LanguagesFile struct {
	Languages []LanguageDef `yaml:"languages"`
}
