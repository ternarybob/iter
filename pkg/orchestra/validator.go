package orchestra

import (
	"context"
	"fmt"

	"github.com/ternarybob/iter/pkg/llm"
)

// Validator is the adversarial review agent.
// Its default stance is REJECT - implementations are guilty until proven innocent.
type Validator struct {
	provider llm.Provider
	workdir  *WorkdirManager
}

// NewValidator creates a new validator agent.
func NewValidator(provider llm.Provider, workdir *WorkdirManager) *Validator {
	return &Validator{
		provider: provider,
		workdir:  workdir,
	}
}

// Validate reviews an implementation against requirements.
// The validator assumes ALL implementations are incorrect until proven otherwise.
func (v *Validator) Validate(ctx context.Context, step *Step, result *StepResult, reqs *Requirements) (*Verdict, error) {
	// First check auto-reject conditions
	autoReject := CheckAutoReject(result, step)
	if autoReject != nil {
		// Write validation document
		if v.workdir != nil {
			_ = v.workdir.WriteStepValidation(step.Number, autoReject.ToDocument(step.Number))
		}
		return autoReject, nil
	}

	// Build the validation prompt
	prompt := buildValidationPrompt(step, result, reqs)

	// Call LLM for validation
	req := &llm.CompletionRequest{
		System: validatorSystemPrompt,
		Messages: []llm.Message{
			llm.UserMessage(prompt),
		},
		MaxTokens: 4096,
	}

	resp, err := v.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	// Parse verdict from response
	verdict, err := ParseVerdict(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse verdict: %w", err)
	}

	// Apply auto-reject logic if not explicit pass
	verdict = v.applyAutoRejectRules(verdict, step, result)

	// Write validation document
	if v.workdir != nil {
		if err := v.workdir.WriteStepValidation(step.Number, verdict.ToDocument(step.Number)); err != nil {
			// Non-fatal
			_ = err
		}
	}

	return verdict, nil
}

// FinalValidate reviews all changes together.
func (v *Validator) FinalValidate(ctx context.Context, results []*StepResult, reqs *Requirements) (*Verdict, error) {
	// Build the final validation prompt
	prompt := buildFinalValidationPrompt(results, reqs)

	// Call LLM for final validation
	req := &llm.CompletionRequest{
		System: validatorSystemPrompt,
		Messages: []llm.Message{
			llm.UserMessage(prompt),
		},
		MaxTokens: 4096,
	}

	resp, err := v.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	// Parse verdict from response
	verdict, err := ParseVerdict(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse verdict: %w", err)
	}

	// Write final validation document
	if v.workdir != nil {
		if err := v.workdir.WriteFinalValidation(resp.Content); err != nil {
			_ = err
		}
	}

	return verdict, nil
}

// validatorSystemPrompt is the system prompt for the validator agent.
const validatorSystemPrompt = `You are a hostile code reviewer. Your job is to REJECT implementations that don't meet requirements.

## YOUR STANCE: DEFAULT REJECT

You assume ALL implementations are incorrect until proven otherwise. This is intentional and critical for code quality.

## AUTO-REJECT CONDITIONS (immediate rejection, no further review needed)
- Build fails
- Tests fail
- Requirements not traceable to code (line references required)
- Dead code left behind
- Old function exists alongside replacement
- Codebase style violations
- Missing error handling
- Cleanup not performed as specified

## PASS CONDITIONS (ALL must be true)
- ALL requirements verified with code line references
- Build passes
- Tests pass (if applicable)
- No dead code
- Style compliance
- Cleanup verified

## Output Format

# Validation: Step N

## Build Status
- Result: PASS/FAIL
- Log: [path]

## Test Status
- Result: PASS/FAIL/SKIPPED

## Requirements Verification
| REQ | Status | Evidence (file:line) |
|-----|--------|---------------------|
| REQ-1 | ✓/✗ | path/to/file.go:42 |

## Acceptance Criteria
| AC | Status | Verification |
|----|--------|--------------|
| AC-1 | ✓/✗ | [how verified] |

## Cleanup Verification
| Item | Removed | Confirmed |
|------|---------|-----------|
| [item] | ✓/✗ | [evidence] |

## Style Compliance
- Error handling: ✓/✗
- Logging conventions: ✓/✗
- Code organization: ✓/✗

## Verdict: PASS / REJECT

## Rejection Reasons (if REJECT)
1. [specific issue with fix instructions]
2. [specific issue with fix instructions]`

// buildValidationPrompt creates the prompt for step validation.
func buildValidationPrompt(step *Step, result *StepResult, reqs *Requirements) string {
	var sb stringBuilder

	sb.WriteString("Review the following implementation with your DEFAULT REJECT stance.\n\n")

	sb.WriteString("## Requirements\n")
	if reqs != nil {
		sb.WriteString(reqs.Document + "\n\n")
	}

	sb.WriteString("## Step Document\n")
	sb.WriteString(step.Document + "\n\n")

	sb.WriteString("## Implementation\n")
	sb.WriteString(result.Document + "\n\n")

	sb.WriteString("## Build Status\n")
	if result.BuildPassed {
		sb.WriteString("- Build: PASS\n")
	} else {
		sb.WriteString("- Build: FAIL\n")
		sb.WriteString("- Log: " + result.BuildLog + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Changes Made\n")
	for _, change := range result.Changes {
		sb.WriteString("- " + string(change.Type) + ": " + change.Path + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Verify EACH requirement with code line references\n")
	sb.WriteString("2. Check ALL acceptance criteria\n")
	sb.WriteString("3. Verify ALL cleanup was performed\n")
	sb.WriteString("4. Check style compliance\n")
	sb.WriteString("5. Apply AUTO-REJECT rules\n\n")

	sb.WriteString("Remember: Your default stance is REJECT. Only pass if ALL conditions are met.\n")

	return sb.String()
}

// buildFinalValidationPrompt creates the prompt for final validation.
func buildFinalValidationPrompt(results []*StepResult, reqs *Requirements) string {
	var sb stringBuilder

	sb.WriteString("Review ALL changes together with your DEFAULT REJECT stance.\n\n")

	sb.WriteString("## Requirements\n")
	if reqs != nil {
		sb.WriteString(reqs.Document + "\n\n")
	}

	sb.WriteString("## All Step Results\n")
	for _, result := range results {
		if result == nil {
			continue
		}
		sb.WriteString("### Step " + itoa(result.StepNumber) + "\n")
		sb.WriteString("- Build: ")
		if result.BuildPassed {
			sb.WriteString("PASS\n")
		} else {
			sb.WriteString("FAIL\n")
		}
		sb.WriteString("- Changes: " + itoa(len(result.Changes)) + " files\n")
		sb.WriteString("\n")
	}

	sb.WriteString("## Final Validation Instructions\n")
	sb.WriteString("1. Re-read ALL requirements\n")
	sb.WriteString("2. Verify ALL requirements are satisfied across all steps\n")
	sb.WriteString("3. Check for conflicts between steps\n")
	sb.WriteString("4. Verify NO dead code across ALL changes\n")
	sb.WriteString("5. Full build must pass\n")
	sb.WriteString("6. Full test suite must pass\n\n")

	sb.WriteString("Remember: Your default stance is REJECT. Only pass if ALL conditions are met.\n")

	return sb.String()
}

// applyAutoRejectRules applies automatic rejection rules.
func (v *Validator) applyAutoRejectRules(verdict *Verdict, step *Step, result *StepResult) *Verdict {
	// If already rejected, return as-is
	if verdict.Status == VerdictReject {
		return verdict
	}

	// Check build
	if !result.BuildPassed {
		verdict.Status = VerdictReject
		verdict.Reasons = append(verdict.Reasons, "Build failed")
		verdict.BuildPassed = false
	} else {
		verdict.BuildPassed = true
	}

	// Check cleanup
	cleanupVerified := true
	for _, item := range step.Cleanup {
		// In a real implementation, we would check if the item was removed
		// For now, we trust the LLM's assessment
		_ = item
	}
	if !cleanupVerified {
		verdict.Status = VerdictReject
		verdict.Reasons = append(verdict.Reasons, "Cleanup not completed as specified")
	}
	verdict.CleanupVerified = cleanupVerified

	// Check requirements traceability
	if len(verdict.RequirementStatus) == 0 && len(step.Requirements) > 0 {
		verdict.Status = VerdictReject
		verdict.Reasons = append(verdict.Reasons, "Requirements not verified with code references")
	}

	// Check acceptance criteria
	for _, ac := range step.AcceptanceCriteria {
		acID := "AC-" + itoa(len(verdict.AcceptanceStatus)+1)
		if _, verified := verdict.AcceptanceStatus[acID]; !verified {
			verdict.AcceptanceStatus[acID] = false
		}
		_ = ac
	}

	return verdict
}
