package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const skillFileName = "SKILL.md"

// Skill is a loaded agent skill (Cursor-style SKILL.md).
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Body        string `json:"body"`
}

// Catalog indexes skills discovered under configured directories.
type Catalog struct {
	skills []Skill
	byName map[string]Skill
}

// LoadCatalog scans dirs for */SKILL.md (or SKILL.md at dir root). Missing dirs are skipped.
func LoadCatalog(dirs []string) *Catalog {
	seen := make(map[string]Skill)
	for _, root := range dirs {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Base(path), skillFileName) {
				return nil
			}
			sk, err := parseSkillFile(path)
			if err != nil || sk.Name == "" {
				return nil
			}
			if prev, ok := seen[sk.Name]; ok {
				// Prefer project-local over global when names collide (later dirs win).
				if len(path) < len(prev.Path) {
					return nil
				}
			}
			seen[sk.Name] = sk
			return nil
		})
	}
	c := &Catalog{byName: seen}
	for _, sk := range seen {
		c.skills = append(c.skills, sk)
	}
	sort.Slice(c.skills, func(i, j int) bool { return c.skills[i].Name < c.skills[j].Name })
	return c
}

func parseSkillFile(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	body := strings.TrimSpace(string(data))
	name := filepath.Base(filepath.Dir(path))
	if name == "." || name == "skills" || name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	desc := extractDescription(body)
	return Skill{
		Name:        name,
		Description: desc,
		Path:        path,
		Body:        body,
	}, nil
}

func extractDescription(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		if len(line) > 160 {
			line = line[:160] + "…"
		}
		return line
	}
	return "Agent skill"
}

// List returns all skills sorted by name.
func (c *Catalog) List() []Skill {
	if c == nil {
		return nil
	}
	out := make([]Skill, len(c.skills))
	copy(out, c.skills)
	return out
}

// Get returns a skill by name (case-insensitive).
func (c *Catalog) Get(name string) (Skill, bool) {
	if c == nil {
		return Skill{}, false
	}
	if sk, ok := c.byName[name]; ok {
		return sk, true
	}
	lower := strings.ToLower(name)
	for k, sk := range c.byName {
		if strings.ToLower(k) == lower {
			return sk, true
		}
	}
	return Skill{}, false
}

// Count returns the number of loaded skills.
func (c *Catalog) Count() int {
	if c == nil {
		return 0
	}
	return len(c.skills)
}
