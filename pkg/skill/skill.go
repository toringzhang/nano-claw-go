package skill

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

/*
## Skill Structure

Each skill is a directory containing a SKILL.md file with YAML frontmatter:

```
skills
└── my-skill
    └── SKILL.md       # Required: instructions + metadata
    └── scripts/       # Optional: executable code
    └── references/    # Optional: documentation
    └── assets/        # Optional: templates, resources
```

SKILL.md format:
```markdown
---
name: pdf-processing
Description: Extract PDF text, fill forms, merge files. Use when handling PDFs.
---

# PDF Processing

## When to use this skill
Use this skill when the user needs to work with PDF files...

## How to extract text
1. Use pdfplumber for text extraction...

## How to fill forms
...
```
*/

const (
	skillMarkdown   = "SKILL.md"
	metadataPattern = `(?m)^---([\s\S]*?)---\n?([\s\S]*)`
)

type SkillLoader interface {
	Load() error
	Append(path string)
	Skill(name string) string
	Prompt() string
}

type Skill struct {
	Path        string
	Name        string
	Description string
	License     string
	Content     string
}

type skillLoader struct {
	skillDir string
	mutex    sync.RWMutex
	skills   map[string]Skill
}

func NewSkillLoader(skillDir string) SkillLoader {
	return &skillLoader{
		skillDir: skillDir,
		skills:   make(map[string]Skill),
	}
}

func (sl *skillLoader) Load() error {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	f, err := os.Stat(sl.skillDir)
	if err != nil {
		return fmt.Errorf("load skills failed, %v", err)
	}
	if !f.IsDir() {
		return fmt.Errorf("load skills failed, %sl is not a directory", sl.skillDir)
	}
	entries, err := os.ReadDir(sl.skillDir)
	if err != nil {
		return fmt.Errorf("load skills failed, %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(sl.skillDir, entry.Name())
		skill, err := readSkill(skillDir)
		if err != nil {
			log.Printf("load skills %s failed, %v", skillDir, err)
			continue
		}
		sl.skills[skill.Name] = *skill
	}
	return nil
}

func (sl *skillLoader) Append(path string) {
	skill, err := readSkill(path)
	if err != nil {
		log.Printf("load skill %s failed, %v", path, err)
		return
	}
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	sl.skills[skill.Name] = *skill
}

func (sl *skillLoader) Skill(name string) string {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	skill, ok := sl.skills[name]
	if !ok {
		return fmt.Sprintf("Error: Unknown skill \"%s\".", name)
	}

	return fmt.Sprintf("<skill name=\"%s\">\n%s\n</skill>", name, skill.Content)
}

func (sl *skillLoader) Prompt() string {

	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	prompt := func() string {
		if len(sl.skills) == 0 {
			return "(No skills loaded.)"
		}
		output := ""
		for _, skill := range sl.skills {
			output += fmt.Sprintf("\n  - %s: %s", skill.Name, skill.Description)
		}
		return output
	}()

	return fmt.Sprintf("Use load_skill for specialized knowledge.\nSkills: %s\n", prompt)
}

func readSkill(path string) (*Skill, error) {
	skillPath := filepath.Join(path, skillMarkdown)
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}
	skill := Skill{
		Path: skillPath,
	}

	re := regexp.MustCompile(metadataPattern)
	doc := re.FindStringSubmatch(string(content))
	if len(doc) < 3 {
		return nil, fmt.Errorf("%s format error", skillPath)
	}
	for _, item := range strings.Split(doc[1], "\n") {
		kv := strings.Split(strings.TrimSpace(item), ":")
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "name" {
			skill.Name = strings.ToLower(strings.TrimSpace(kv[1]))
		}
		if kv[0] == "description" {
			skill.Description = strings.TrimSpace(kv[1])
		}
		if kv[0] == "license" {
			skill.License = strings.TrimSpace(kv[1])
		}
	}
	if len(skill.Name) == 0 || len(skill.Description) == 0 {
		return nil, fmt.Errorf("name/description must define")
	}
	skill.Content = doc[2]
	return &skill, nil
}
