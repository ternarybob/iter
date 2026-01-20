package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeDir represents the .claude directory structure.
type ClaudeDir struct {
	// Root is the .claude directory path.
	Root string

	// Settings is the loaded settings.json.
	Settings *FileConfig

	// Commands are loaded command definitions.
	Commands map[string]*Command

	// Skills are loaded skill definitions.
	Skills map[string]*SkillDef
}

// Command represents a slash command from .claude/commands/.
type Command struct {
	// Name is the command name (filename without extension).
	Name string

	// Content is the command definition.
	Content string

	// Path is the file path.
	Path string
}

// LoadClaudeDir loads the .claude directory from a project root.
func LoadClaudeDir(projectRoot string) (*ClaudeDir, error) {
	claudeRoot := filepath.Join(projectRoot, ".claude")

	// Check if .claude directory exists
	info, err := os.Stat(claudeRoot)
	if os.IsNotExist(err) {
		// Return empty ClaudeDir if not found
		return &ClaudeDir{
			Root:     claudeRoot,
			Commands: make(map[string]*Command),
			Skills:   make(map[string]*SkillDef),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat .claude: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf(".claude is not a directory")
	}

	cd := &ClaudeDir{
		Root:     claudeRoot,
		Commands: make(map[string]*Command),
		Skills:   make(map[string]*SkillDef),
	}

	// Load settings.json
	settingsPath := filepath.Join(claudeRoot, "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		var settings FileConfig
		if err := parseJSON(data, &settings); err != nil {
			return nil, fmt.Errorf("parse settings.json: %w", err)
		}
		cd.Settings = &settings
	}

	// Load commands
	commandsDir := filepath.Join(claudeRoot, "commands")
	if entries, err := os.ReadDir(commandsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".md")
			path := filepath.Join(commandsDir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			cd.Commands[name] = &Command{
				Name:    name,
				Content: string(content),
				Path:    path,
			}
		}
	}

	// Load skills
	skillsDir := filepath.Join(claudeRoot, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			if content, err := os.ReadFile(skillPath); err == nil {
				skill, err := ParseSkillMD(string(content))
				if err != nil {
					continue
				}
				skill.Path = skillPath
				cd.Skills[entry.Name()] = skill
			}
		}
	}

	return cd, nil
}

// EnsureWorkdir creates the workdir directory if it doesn't exist.
func (cd *ClaudeDir) EnsureWorkdir() (string, error) {
	workdirPath := filepath.Join(cd.Root, "workdir")
	if err := os.MkdirAll(workdirPath, 0755); err != nil {
		return "", fmt.Errorf("create workdir: %w", err)
	}
	return workdirPath, nil
}

// GetCommand returns a command by name.
func (cd *ClaudeDir) GetCommand(name string) (*Command, bool) {
	cmd, ok := cd.Commands[name]
	return cmd, ok
}

// GetSkill returns a skill by name.
func (cd *ClaudeDir) GetSkill(name string) (*SkillDef, bool) {
	skill, ok := cd.Skills[name]
	return skill, ok
}

// ListCommands returns all command names.
func (cd *ClaudeDir) ListCommands() []string {
	names := make([]string, 0, len(cd.Commands))
	for name := range cd.Commands {
		names = append(names, name)
	}
	return names
}

// ListSkills returns all skill names.
func (cd *ClaudeDir) ListSkills() []string {
	names := make([]string, 0, len(cd.Skills))
	for name := range cd.Skills {
		names = append(names, name)
	}
	return names
}

// InitClaudeDir initializes a new .claude directory structure.
func InitClaudeDir(projectRoot string) error {
	claudeRoot := filepath.Join(projectRoot, ".claude")

	// Create directories
	dirs := []string{
		claudeRoot,
		filepath.Join(claudeRoot, "commands"),
		filepath.Join(claudeRoot, "skills"),
		filepath.Join(claudeRoot, "workdir"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Create default settings.json
	settingsPath := filepath.Join(claudeRoot, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		settings := `{
  "project": {
    "name": "",
    "ignorePatterns": ["vendor/", "node_modules/", ".git/"],
    "indexPatterns": ["*.go", "*.ts", "*.py", "*.js"]
  },
  "models": {
    "planning": "claude-sonnet-4-20250514",
    "execution": "claude-sonnet-4-20250514",
    "validation": "claude-sonnet-4-20250514"
  },
  "loop": {
    "maxIterations": 100,
    "rateLimitPerHour": 100,
    "iterationTimeout": "15m",
    "cooldown": "5s",
    "maxValidationRetries": 5,
    "parallelSteps": true
  },
  "exit": {
    "requireExplicitSignal": true,
    "completionThreshold": 2,
    "maxConsecutiveNoProgress": 3
  },
  "circuit": {
    "noProgressThreshold": 3,
    "sameErrorThreshold": 5,
    "recoveryTimeout": "5m"
  },
  "monitor": {
    "enabled": false,
    "port": 8080
  }
}
`
		if err := os.WriteFile(settingsPath, []byte(settings), 0644); err != nil {
			return fmt.Errorf("write settings.json: %w", err)
		}
	}

	return nil
}

// parseJSON is a simple JSON parser wrapper.
func parseJSON(data []byte, v interface{}) error {
	// Use encoding/json
	return unmarshalJSON(data, v)
}
