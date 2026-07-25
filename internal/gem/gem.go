package gem

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// SourceType identifies where a gem comes from.
type SourceType string

const (
	SourceRubygems SourceType = "rubygems"
	SourceGit      SourceType = "git"
	SourcePath     SourceType = "path"
)

// Dependency is a gem dependency with its version constraint.
type Dependency struct {
	Name       string `json:"name"`
	Constraint string `json:"constraint,omitempty"`
}

// Gem holds parsed information about a single gem.
type Gem struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Source       SourceType   `json:"source"`
	SourceURL    string       `json:"source_url"`
	Revision     string       `json:"revision,omitempty"`
	Branch       string       `json:"branch,omitempty"`
	Tag          string       `json:"tag,omitempty"`
	Ref          string       `json:"ref,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

// Lockfile holds all parsed data from a Gemfile.lock.
type Lockfile struct {
	gems           map[string]*Gem
	rubyVersion    string
	bundlerVersion string
	platforms      []string
}

// RubyVersion returns the Ruby version recorded in Gemfile.lock.
func (l *Lockfile) RubyVersion() string { return l.rubyVersion }

// BundlerVersion returns the Bundler version recorded in Gemfile.lock.
func (l *Lockfile) BundlerVersion() string { return l.bundlerVersion }

// Platforms returns the platforms recorded in Gemfile.lock.
func (l *Lockfile) Platforms() []string {
	return append([]string(nil), l.platforms...)
}

// List returns all gems sorted alphabetically by name.
func (l *Lockfile) List() []Gem {
	gems := make([]Gem, 0, len(l.gems))
	for _, g := range l.gems {
		gems = append(gems, *g)
	}
	sort.Slice(gems, func(i, j int) bool {
		return gems[i].Name < gems[j].Name
	})
	return gems
}

// Find returns the gem with the given name (case-insensitive), or nil if not found.
func (l *Lockfile) Find(name string) *Gem {
	lower := strings.ToLower(name)
	for k, g := range l.gems {
		if strings.ToLower(k) == lower {
			return g
		}
	}
	return nil
}

var (
	reGemEntry = regexp.MustCompile(`^    (\S+) \(([^)]+)\)$`)
	reGemDep   = regexp.MustCompile(`^      (\S+)(?: \(([^)]+)\))?$`)
	reRemote   = regexp.MustCompile(`^  remote: (.+)$`)
	reRevision = regexp.MustCompile(`^  revision: (.+)$`)
	reBranch   = regexp.MustCompile(`^  branch: (.+)$`)
	reTag      = regexp.MustCompile(`^  tag: (.+)$`)
	reRef      = regexp.MustCompile(`^  ref: (.+)$`)
)

type sectionType int

const (
	sectionNone sectionType = iota
	sectionGEM
	sectionGIT
	sectionPATH
	sectionOther
)

// sectionContext holds metadata accumulated for the current GEM/GIT/PATH block.
type sectionContext struct {
	kind      sectionType
	remote    string
	revision  string
	branch    string
	tag       string
	ref       string
	inSpecs   bool
	sourceURL string // normalized: for GEM this is the remote, for GIT/PATH also the remote
}

func (c *sectionContext) reset(kind sectionType) {
	c.kind = kind
	c.remote = ""
	c.revision = ""
	c.branch = ""
	c.tag = ""
	c.ref = ""
	c.inSpecs = false
	c.sourceURL = ""
}

func (c *sectionContext) sourceType() SourceType {
	switch c.kind {
	case sectionGEM:
		return SourceRubygems
	case sectionGIT:
		return SourceGit
	case sectionPATH:
		return SourcePath
	default:
		return SourceRubygems
	}
}

// Parse reads a Gemfile.lock file and returns the parsed Lockfile.
func Parse(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening Gemfile.lock: %w", err)
	}

	lf := &Lockfile{gems: make(map[string]*Gem)}
	parseMetadata(data, lf)
	ctx := &sectionContext{}
	var currentGem *Gem

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()

		// Blank line: reset section context (sections are separated by blank lines)
		if strings.TrimSpace(line) == "" {
			ctx.reset(sectionNone)
			currentGem = nil
			continue
		}

		// Unindented line: section header
		if len(line) > 0 && line[0] != ' ' {
			currentGem = nil
			switch strings.TrimSpace(line) {
			case "GEM":
				ctx.reset(sectionGEM)
			case "GIT":
				ctx.reset(sectionGIT)
			case "PATH":
				ctx.reset(sectionPATH)
			default:
				ctx.reset(sectionOther)
			}
			continue
		}

		// Only parse details inside GEM/GIT/PATH sections
		if ctx.kind == sectionNone || ctx.kind == sectionOther {
			continue
		}

		// 2-space indented lines: metadata or "specs:"
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "specs:" {
				ctx.inSpecs = true
				continue
			}
			if m := reRemote.FindStringSubmatch(line); m != nil {
				ctx.remote = strings.TrimSpace(m[1])
				ctx.sourceURL = ctx.remote
				continue
			}
			if m := reRevision.FindStringSubmatch(line); m != nil {
				ctx.revision = strings.TrimSpace(m[1])
				continue
			}
			if m := reBranch.FindStringSubmatch(line); m != nil {
				ctx.branch = strings.TrimSpace(m[1])
				continue
			}
			if m := reTag.FindStringSubmatch(line); m != nil {
				ctx.tag = strings.TrimSpace(m[1])
				continue
			}
			if m := reRef.FindStringSubmatch(line); m != nil {
				ctx.ref = strings.TrimSpace(m[1])
				continue
			}
			continue
		}

		if !ctx.inSpecs {
			continue
		}

		// 4-space indent: gem entry
		if m := reGemEntry.FindStringSubmatch(line); m != nil {
			name := m[1]
			version := m[2]
			g := &Gem{
				Name:      name,
				Version:   version,
				Source:    ctx.sourceType(),
				SourceURL: ctx.sourceURL,
				Revision:  ctx.revision,
				Branch:    ctx.branch,
				Tag:       ctx.tag,
				Ref:       ctx.ref,
			}
			lf.gems[name] = g
			currentGem = g
			continue
		}

		// 6-space indent: dependency of the current gem
		if currentGem != nil {
			if m := reGemDep.FindStringSubmatch(line); m != nil {
				dep := Dependency{Name: m[1]}
				if len(m) > 2 {
					dep.Constraint = m[2]
				}
				currentGem.Dependencies = append(currentGem.Dependencies, dep)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning Gemfile.lock: %w", err)
	}

	return lf, nil
}

func parseMetadata(data []byte, lf *Lockfile) {
	var section string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" && line[0] != ' ' {
			section = strings.TrimSpace(line)
			continue
		}
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		switch section {
		case "PLATFORMS":
			lf.platforms = append(lf.platforms, value)
		case "RUBY VERSION":
			lf.rubyVersion = strings.TrimPrefix(value, "ruby ")
		case "BUNDLED WITH":
			lf.bundlerVersion = value
		}
	}
}
