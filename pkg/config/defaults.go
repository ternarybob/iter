package config

import (
	"encoding/json"

	"github.com/ternarybob/iter/pkg/sdk"
)

// DefaultConfig returns the default configuration.
func DefaultConfig() *sdk.Config {
	return &sdk.Config{
		Project: sdk.ProjectConfig{
			RootDir:        ".",
			IgnorePatterns: DefaultIgnorePatterns(),
			IndexPatterns:  DefaultIndexPatterns(),
		},
		Models: sdk.ModelConfig{
			Planning:   DefaultPlanningModel,
			Execution:  DefaultExecutionModel,
			Validation: DefaultValidationModel,
		},
		Loop: sdk.LoopConfig{
			MaxIterations:        DefaultMaxIterations,
			RateLimitPerHour:     DefaultRateLimitPerHour,
			IterationTimeout:     DefaultIterationTimeout,
			Cooldown:             DefaultCooldown,
			MaxValidationRetries: DefaultMaxValidationRetries,
			ParallelSteps:        true,
		},
		Exit: sdk.ExitConfig{
			RequireExplicitSignal:    true,
			CompletionThreshold:      DefaultCompletionThreshold,
			MaxConsecutiveNoProgress: DefaultMaxConsecutiveNoProgress,
			MaxConsecutiveErrors:     DefaultMaxConsecutiveErrors,
		},
		Circuit: sdk.CircuitConfig{
			NoProgressThreshold:    DefaultNoProgressThreshold,
			SameErrorThreshold:     DefaultSameErrorThreshold,
			OutputDeclineThreshold: DefaultOutputDeclineThreshold,
			RecoveryTimeout:        DefaultRecoveryTimeout,
		},
		Monitor: sdk.MonitorConfig{
			Enabled: false,
			Port:    DefaultMonitorPort,
		},
	}
}

// Default model identifiers
const (
	DefaultPlanningModel   = "claude-sonnet-4-20250514"
	DefaultExecutionModel  = "claude-sonnet-4-20250514"
	DefaultValidationModel = "claude-sonnet-4-20250514"
)

// Default loop settings
const (
	DefaultMaxIterations        = 100
	DefaultRateLimitPerHour     = 100
	DefaultIterationTimeout     = "15m"
	DefaultCooldown             = "5s"
	DefaultMaxValidationRetries = 5
)

// Default exit settings
const (
	DefaultCompletionThreshold      = 2
	DefaultMaxConsecutiveNoProgress = 3
	DefaultMaxConsecutiveErrors     = 5
)

// Default circuit breaker settings
const (
	DefaultNoProgressThreshold    = 3
	DefaultSameErrorThreshold     = 5
	DefaultOutputDeclineThreshold = 70
	DefaultRecoveryTimeout        = "5m"
)

// Default monitor settings
const (
	DefaultMonitorPort = 8080
)

// DefaultIgnorePatterns returns the default patterns to ignore during indexing.
func DefaultIgnorePatterns() []string {
	return []string{
		"vendor/",
		"node_modules/",
		".git/",
		".svn/",
		".hg/",
		"__pycache__/",
		".pytest_cache/",
		".mypy_cache/",
		"*.pyc",
		"*.pyo",
		".tox/",
		".eggs/",
		"*.egg-info/",
		"dist/",
		"build/",
		"target/",
		".gradle/",
		".idea/",
		".vscode/",
		".vs/",
		"*.class",
		"*.jar",
		"*.war",
		"*.o",
		"*.a",
		"*.so",
		"*.dylib",
		"*.exe",
		"*.dll",
		"coverage/",
		".coverage",
		"htmlcov/",
		".nyc_output/",
		"*.min.js",
		"*.min.css",
		"*.map",
		".next/",
		".nuxt/",
		".output/",
		".vercel/",
		".netlify/",
	}
}

// DefaultIndexPatterns returns the default patterns to index.
func DefaultIndexPatterns() []string {
	return []string{
		// Go
		"*.go",
		// JavaScript/TypeScript
		"*.js",
		"*.jsx",
		"*.ts",
		"*.tsx",
		"*.mjs",
		"*.cjs",
		// Python
		"*.py",
		"*.pyi",
		// Java/Kotlin
		"*.java",
		"*.kt",
		"*.kts",
		// C/C++
		"*.c",
		"*.h",
		"*.cpp",
		"*.hpp",
		"*.cc",
		"*.hh",
		// Rust
		"*.rs",
		// Ruby
		"*.rb",
		// PHP
		"*.php",
		// Shell
		"*.sh",
		"*.bash",
		"*.zsh",
		// Configuration
		"*.yaml",
		"*.yml",
		"*.json",
		"*.toml",
		// Markup
		"*.md",
		"*.markdown",
		// Docker
		"Dockerfile",
		"*.dockerfile",
		"docker-compose*.yml",
		// Kubernetes
		"*.k8s.yaml",
		"*.k8s.yml",
		// CI/CD
		".github/workflows/*.yml",
		".gitlab-ci.yml",
		"Jenkinsfile",
		// Terraform
		"*.tf",
		"*.tfvars",
	}
}

// unmarshalJSON wraps json.Unmarshal.
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
