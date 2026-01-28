package iter

import _ "embed"

// Embedded prompt files

//go:embed prompts/system.md
var PromptSystem string

//go:embed prompts/architect.md
var PromptArchitect string

//go:embed prompts/worker.md
var PromptWorker string

//go:embed prompts/validator.md
var PromptValidator string

//go:embed templates/CLAUDE.md.tmpl
var ClaudeMDTemplate string
