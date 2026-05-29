package registry

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
	"gopkg.in/yaml.v3"
)

type Registry struct {
	langs   map[string]*config.LanguageDef
	ordered []*config.LanguageDef // preserved YAML insertion order for All()
}

// Load reads and validates the language YAML file. Returns an error if any entry is malformed.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading language file %s: %w", path, err)
	}

	var lf config.LanguagesFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // reject unknown keys to catch typos early
	if err := dec.Decode(&lf); err != nil {
		return nil, fmt.Errorf("parsing language file: %w", err)
	}

	if len(lf.Languages) == 0 {
		return nil, fmt.Errorf("language file contains no languages")
	}

	r := &Registry{
		langs:   make(map[string]*config.LanguageDef, len(lf.Languages)),
		ordered: make([]*config.LanguageDef, 0, len(lf.Languages)),
	}
	for i := range lf.Languages {
		lang := &lf.Languages[i]
		if err := validateLang(lang); err != nil {
			return nil, fmt.Errorf("language[%d] %q: %w", i, lang.ID, err)
		}
		if _, dup := r.langs[lang.ID]; dup {
			return nil, fmt.Errorf("duplicate language id %q", lang.ID)
		}
		r.langs[lang.ID] = lang
		r.ordered = append(r.ordered, lang)
	}
	return r, nil
}

// Get returns the LanguageDef for id, or nil if not found.
func (r *Registry) Get(id string) *config.LanguageDef {
	return r.langs[id]
}

// All returns all language definitions in YAML insertion order.
func (r *Registry) All() []*config.LanguageDef {
	return r.ordered
}

// Len returns the number of registered languages.
func (r *Registry) Len() int { return len(r.langs) }

// MaxJobDuration returns the upper-bound wall time for any job across all languages,
// used to set the HTTP server WriteTimeout.
func (r *Registry) MaxJobDuration(maxTests int) time.Duration {
	var maxBuild, maxRun int
	for _, lang := range r.langs {
		if lang.Build != nil && lang.Build.Limits.WallTimeS > maxBuild {
			maxBuild = lang.Build.Limits.WallTimeS
		}
		if lang.Run.Limits.WallTimeS > maxRun {
			maxRun = lang.Run.Limits.WallTimeS
		}
	}
	total := maxBuild + maxTests*maxRun + 30
	return time.Duration(total) * time.Second
}

func validateLang(l *config.LanguageDef) error {
	if l.ID == "" {
		return fmt.Errorf("id is required")
	}
	if l.Name == "" {
		return fmt.Errorf("name is required")
	}

	if l.SourceFilenameStrategy != "" && l.SourceFilenameStrategy != "from_request" {
		return fmt.Errorf("source_filename_strategy must be \"from_request\" or empty, got %q", l.SourceFilenameStrategy)
	}
	if l.SourceFilenameStrategy != "from_request" && l.SourceFilename == "" {
		return fmt.Errorf("source_filename is required when source_filename_strategy is not from_request")
	}
	if l.ArtifactFilenameStrategy != "" && l.ArtifactFilenameStrategy != "from_request" {
		return fmt.Errorf("artifact_filename_strategy must be \"from_request\" or empty, got %q", l.ArtifactFilenameStrategy)
	}

	if l.Run.Cmd == "" {
		return fmt.Errorf("run.cmd is required")
	}
	if err := checkLimits("run", l.Run.Limits); err != nil {
		return err
	}
	if err := checkArgs("run", l.Run.Args); err != nil {
		return err
	}

	if l.Build != nil {
		if l.Build.Cmd == "" {
			return fmt.Errorf("build.cmd is required when build block is present")
		}
		if err := checkLimits("build", l.Build.Limits); err != nil {
			return err
		}
		if err := checkArgs("build", l.Build.Args); err != nil {
			return err
		}
	}

	return nil
}

// checkLimits rejects limits where any field is zero or negative.
func checkLimits(phase string, limits config.LimitsDef) error {
	if limits.WallTimeS <= 0 {
		return fmt.Errorf("%s.limits.wall_time_s must be > 0", phase)
	}
	if limits.MemoryKB <= 0 {
		return fmt.Errorf("%s.limits.memory_kb must be > 0", phase)
	}
	if limits.MaxProcesses <= 0 {
		return fmt.Errorf("%s.limits.max_processes must be > 0", phase)
	}
	return nil
}

// checkArgs rejects args containing unknown {{...}} placeholders.
// Valid tokens are {{source}}, {{artifact}}, and {{flags}}.
func checkArgs(phase string, args []string) error {
	for _, arg := range args {
		s := arg
		for {
			i := strings.Index(s, "{{")
			if i == -1 {
				break
			}
			j := strings.Index(s[i:], "}}")
			if j == -1 {
				return fmt.Errorf("%s.args: unclosed '{{' in %q", phase, arg)
			}
			token := s[i : i+j+2]
			if token != "{{source}}" && token != "{{artifact}}" && token != "{{flags}}" {
				return fmt.Errorf("%s.args: unknown placeholder %s (valid: {{source}}, {{artifact}}, {{flags}})", phase, token)
			}
			s = s[i+j+2:]
		}
	}
	return nil
}
