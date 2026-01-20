package agent

import (
	"io/fs"
	"log/slog"
	"os"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Option configures an Agent.
type Option func(*Agent) error

// WithConfig sets the agent configuration.
func WithConfig(config *sdk.Config) Option {
	return func(a *Agent) error {
		a.config = config
		return nil
	}
}

// WithLogger sets the agent's logger.
func WithLogger(logger *slog.Logger) Option {
	return func(a *Agent) error {
		a.logger = logger
		return nil
	}
}

// WithWorkDir sets the working directory.
func WithWorkDir(dir string) Option {
	return func(a *Agent) error {
		a.workDir = dir
		// Default to os.DirFS for the working directory
		a.fs = os.DirFS(dir)
		return nil
	}
}

// WithFS sets the file system (primarily for testing).
func WithFS(fsys fs.FS) Option {
	return func(a *Agent) error {
		a.fs = fsys
		return nil
	}
}

// WithLLM sets the LLM router.
func WithLLM(llm sdk.LLMRouter) Option {
	return func(a *Agent) error {
		a.llm = llm
		return nil
	}
}

// WithIndex sets the codebase index.
func WithIndex(index sdk.Index) Option {
	return func(a *Agent) error {
		a.index = index
		return nil
	}
}

// WithSession sets the session store.
func WithSession(session sdk.Session) Option {
	return func(a *Agent) error {
		a.session = session
		return nil
	}
}

// WithOrchestrator sets the multi-agent orchestrator.
func WithOrchestrator(orchestra sdk.Orchestrator) Option {
	return func(a *Agent) error {
		a.orchestra = orchestra
		return nil
	}
}

// WithWorkdirManager sets the workdir artifact manager.
func WithWorkdirManager(workdir sdk.WorkdirManager) Option {
	return func(a *Agent) error {
		a.workdir = workdir
		return nil
	}
}

// WithDryRun enables dry-run mode (no actual changes).
func WithDryRun(dryRun bool) Option {
	return func(a *Agent) error {
		a.dryRun = dryRun
		return nil
	}
}

// WithCircuitBreaker sets the circuit breaker.
func WithCircuitBreaker(cb *CircuitBreaker) Option {
	return func(a *Agent) error {
		a.circuit = cb
		return nil
	}
}

// WithRateLimiter sets the rate limiter.
func WithRateLimiter(rl *RateLimiter) Option {
	return func(a *Agent) error {
		a.rateLimiter = rl
		return nil
	}
}

// WithMonitor sets the monitoring interface.
func WithMonitor(mon Monitor) Option {
	return func(a *Agent) error {
		a.monitor = mon
		return nil
	}
}

// WithMaxIterations sets the maximum iterations.
func WithMaxIterations(max int) Option {
	return func(a *Agent) error {
		a.config.Loop.MaxIterations = max
		return nil
	}
}

// WithRateLimit sets the hourly rate limit.
func WithRateLimit(perHour int) Option {
	return func(a *Agent) error {
		a.config.Loop.RateLimitPerHour = perHour
		if perHour > 0 {
			a.rateLimiter = NewRateLimiter(perHour)
		}
		return nil
	}
}

// WithPlanningModel sets the model for architect agent.
func WithPlanningModel(model string) Option {
	return func(a *Agent) error {
		a.config.Models.Planning = model
		return nil
	}
}

// WithExecutionModel sets the model for worker agent.
func WithExecutionModel(model string) Option {
	return func(a *Agent) error {
		a.config.Models.Execution = model
		return nil
	}
}

// WithValidationModel sets the model for validator agent.
func WithValidationModel(model string) Option {
	return func(a *Agent) error {
		a.config.Models.Validation = model
		return nil
	}
}

// WithExitConfig sets exit detection configuration.
func WithExitConfig(config sdk.ExitConfig) Option {
	return func(a *Agent) error {
		a.config.Exit = config
		return nil
	}
}

// WithCircuitConfig sets circuit breaker configuration.
func WithCircuitConfig(config sdk.CircuitConfig) Option {
	return func(a *Agent) error {
		a.config.Circuit = config
		return nil
	}
}

// WithProjectConfig sets project configuration.
func WithProjectConfig(config sdk.ProjectConfig) Option {
	return func(a *Agent) error {
		a.config.Project = config
		return nil
	}
}

// WithSkills registers multiple skills at once.
func WithSkills(skills ...sdk.Skill) Option {
	return func(a *Agent) error {
		for _, skill := range skills {
			if err := a.registry.Register(skill); err != nil {
				return err
			}
		}
		return nil
	}
}

// WithHooks registers multiple hooks at once.
func WithHooks(hooks map[sdk.HookType][]sdk.Hook) Option {
	return func(a *Agent) error {
		for hookType, hookList := range hooks {
			for _, hook := range hookList {
				a.hooks.Register(hookType, hook)
			}
		}
		return nil
	}
}

// WithIterationTimeout sets the per-iteration timeout.
func WithIterationTimeout(timeout string) Option {
	return func(a *Agent) error {
		a.config.Loop.IterationTimeout = timeout
		return nil
	}
}

// WithCooldown sets the cooldown between iterations.
func WithCooldown(cooldown string) Option {
	return func(a *Agent) error {
		a.config.Loop.Cooldown = cooldown
		return nil
	}
}

// WithParallelSteps enables/disables parallel step execution.
func WithParallelSteps(enabled bool) Option {
	return func(a *Agent) error {
		a.config.Loop.ParallelSteps = enabled
		return nil
	}
}

// WithMaxValidationRetries sets the maximum validation retries.
func WithMaxValidationRetries(max int) Option {
	return func(a *Agent) error {
		a.config.Loop.MaxValidationRetries = max
		return nil
	}
}
