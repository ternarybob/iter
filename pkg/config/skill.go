package config

import (
	"strings"
)

// SkillDef represents a skill definition from SKILL.md.
type SkillDef struct {
	// Name is the skill name.
	Name string

	// Description explains what the skill does.
	Description string

	// Triggers are patterns that activate this skill.
	Triggers []string

	// RequiredTools lists external tools needed.
	RequiredTools []string

	// Configuration is the YAML configuration block.
	Configuration string

	// Prompts contains LLM prompts for different phases.
	Prompts SkillPrompts

	// Path is the file path (set after loading).
	Path string

	// Raw is the raw markdown content.
	Raw string
}

// SkillPrompts contains prompts for skill execution phases.
type SkillPrompts struct {
	Planning   string
	Execution  string
	Validation string
}

// ParseSkillMD parses a SKILL.md file content.
func ParseSkillMD(content string) (*SkillDef, error) {
	skill := &SkillDef{
		Raw: content,
	}

	// Split into sections
	sections := parseSections(content)

	// Extract skill name from title
	if title, ok := sections["title"]; ok {
		skill.Name = strings.TrimSpace(title)
	}

	// Extract description
	if desc, ok := sections["description"]; ok {
		skill.Description = strings.TrimSpace(desc)
	}

	// Extract triggers
	if triggers, ok := sections["triggers"]; ok {
		skill.Triggers = parseList(triggers)
	}

	// Extract required tools
	if tools, ok := sections["required tools"]; ok {
		skill.RequiredTools = parseList(tools)
	}

	// Extract configuration
	if config, ok := sections["configuration"]; ok {
		skill.Configuration = extractCodeBlock(config, "yaml")
	}

	// Extract prompts
	if prompts, ok := sections["prompts"]; ok {
		skill.Prompts = parsePrompts(prompts)
	}

	return skill, nil
}

// parseSections splits markdown into sections by heading.
func parseSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentSection string
	var currentContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			// Title (h1)
			if currentSection != "" {
				sections[currentSection] = currentContent.String()
			}
			sections["title"] = strings.TrimPrefix(line, "# ")
			currentSection = ""
			currentContent.Reset()
		} else if strings.HasPrefix(line, "## ") {
			// Section heading (h2)
			if currentSection != "" {
				sections[currentSection] = currentContent.String()
			}
			currentSection = strings.ToLower(strings.TrimPrefix(line, "## "))
			currentContent.Reset()
		} else if strings.HasPrefix(line, "### ") {
			// Subsection (h3) - include in current section
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		} else {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != "" {
		sections[currentSection] = currentContent.String()
	}

	return sections
}

// parseList parses a markdown list into string slice.
func parseList(content string) []string {
	var items []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			item := strings.TrimPrefix(line, "- ")
			if item != "" {
				items = append(items, item)
			}
		} else if strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(line, "* ")
			if item != "" {
				items = append(items, item)
			}
		}
	}

	return items
}

// extractCodeBlock extracts a fenced code block with the given language.
func extractCodeBlock(content, lang string) string {
	fence := "```" + lang
	start := strings.Index(content, fence)
	if start == -1 {
		// Try without language
		fence = "```"
		start = strings.Index(content, fence)
		if start == -1 {
			return ""
		}
	}

	start += len(fence)
	// Skip to next line
	for start < len(content) && content[start] != '\n' {
		start++
	}
	start++

	end := strings.Index(content[start:], "```")
	if end == -1 {
		return strings.TrimSpace(content[start:])
	}

	return strings.TrimSpace(content[start : start+end])
}

// parsePrompts extracts prompts from the prompts section.
func parsePrompts(content string) SkillPrompts {
	prompts := SkillPrompts{}

	// Find subsections
	sections := parseSubsections(content)

	if planning, ok := sections["planning prompt"]; ok {
		prompts.Planning = strings.TrimSpace(planning)
	}

	if execution, ok := sections["execution prompt"]; ok {
		prompts.Execution = strings.TrimSpace(execution)
	}

	if validation, ok := sections["validation prompt"]; ok {
		prompts.Validation = strings.TrimSpace(validation)
	}

	return prompts
}

// parseSubsections parses h3 subsections within a section.
func parseSubsections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentSection string
	var currentContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			if currentSection != "" {
				sections[currentSection] = currentContent.String()
			}
			currentSection = strings.ToLower(strings.TrimPrefix(line, "### "))
			currentContent.Reset()
		} else {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	if currentSection != "" {
		sections[currentSection] = currentContent.String()
	}

	return sections
}

// ToSkillMetadata converts SkillDef to sdk.SkillMetadata.
func (sd *SkillDef) ToSkillMetadata() SkillMetadata {
	return SkillMetadata{
		Name:          sd.Name,
		Description:   sd.Description,
		Triggers:      sd.Triggers,
		RequiredTools: sd.RequiredTools,
	}
}

// SkillMetadata is a copy of sdk.SkillMetadata to avoid circular imports.
type SkillMetadata struct {
	Name          string
	Description   string
	Version       string
	Triggers      []string
	RequiredTools []string
	Tags          []string
	Author        string
}

// Validate checks if the skill definition is valid.
func (sd *SkillDef) Validate() error {
	if sd.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if sd.Description == "" {
		return &ValidationError{Field: "description", Message: "description is required"}
	}
	return nil
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
