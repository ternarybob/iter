package orchestra

// VerdictStatus indicates validation outcome.
type VerdictStatus string

const (
	VerdictPass   VerdictStatus = "pass"
	VerdictReject VerdictStatus = "reject"
)

// Verdict is a validation result.
type Verdict struct {
	// Status is pass or reject.
	Status VerdictStatus

	// Reasons are rejection reasons.
	Reasons []string

	// RequirementStatus maps REQ to verification (true = verified).
	RequirementStatus map[string]bool

	// AcceptanceStatus maps AC to verification.
	AcceptanceStatus map[string]bool

	// BuildPassed indicates build success.
	BuildPassed bool

	// TestsPassed indicates test success.
	TestsPassed bool

	// CleanupVerified indicates cleanup completion.
	CleanupVerified bool

	// StyleCompliant indicates style compliance.
	StyleCompliant bool

	// Document is the raw validation document.
	Document string
}

// NewVerdict creates a new verdict.
func NewVerdict(status VerdictStatus) *Verdict {
	return &Verdict{
		Status:            status,
		RequirementStatus: make(map[string]bool),
		AcceptanceStatus:  make(map[string]bool),
	}
}

// Pass creates a passing verdict.
func Pass() *Verdict {
	return NewVerdict(VerdictPass)
}

// Reject creates a rejecting verdict with reasons.
func Reject(reasons ...string) *Verdict {
	v := NewVerdict(VerdictReject)
	v.Reasons = reasons
	return v
}

// WithReason adds a rejection reason.
func (v *Verdict) WithReason(reason string) *Verdict {
	v.Reasons = append(v.Reasons, reason)
	return v
}

// WithBuild sets build status.
func (v *Verdict) WithBuild(passed bool) *Verdict {
	v.BuildPassed = passed
	if !passed && v.Status == VerdictPass {
		v.Status = VerdictReject
		v.Reasons = append(v.Reasons, "Build failed")
	}
	return v
}

// WithTests sets test status.
func (v *Verdict) WithTests(passed bool) *Verdict {
	v.TestsPassed = passed
	if !passed && v.Status == VerdictPass {
		v.Status = VerdictReject
		v.Reasons = append(v.Reasons, "Tests failed")
	}
	return v
}

// WithCleanup sets cleanup verification.
func (v *Verdict) WithCleanup(verified bool) *Verdict {
	v.CleanupVerified = verified
	if !verified && v.Status == VerdictPass {
		v.Status = VerdictReject
		v.Reasons = append(v.Reasons, "Cleanup not completed")
	}
	return v
}

// WithStyle sets style compliance.
func (v *Verdict) WithStyle(compliant bool) *Verdict {
	v.StyleCompliant = compliant
	return v
}

// SetRequirement sets requirement verification status.
func (v *Verdict) SetRequirement(reqID string, verified bool) *Verdict {
	if v.RequirementStatus == nil {
		v.RequirementStatus = make(map[string]bool)
	}
	v.RequirementStatus[reqID] = verified
	return v
}

// SetAcceptance sets acceptance criterion verification status.
func (v *Verdict) SetAcceptance(acID string, verified bool) *Verdict {
	if v.AcceptanceStatus == nil {
		v.AcceptanceStatus = make(map[string]bool)
	}
	v.AcceptanceStatus[acID] = verified
	return v
}

// IsPassing returns true if the verdict is a pass.
func (v *Verdict) IsPassing() bool {
	return v.Status == VerdictPass
}

// IsRejecting returns true if the verdict is a rejection.
func (v *Verdict) IsRejecting() bool {
	return v.Status == VerdictReject
}

// AllRequirementsMet returns true if all requirements are verified.
func (v *Verdict) AllRequirementsMet() bool {
	for _, verified := range v.RequirementStatus {
		if !verified {
			return false
		}
	}
	return true
}

// AllAcceptanceMet returns true if all acceptance criteria are verified.
func (v *Verdict) AllAcceptanceMet() bool {
	for _, verified := range v.AcceptanceStatus {
		if !verified {
			return false
		}
	}
	return true
}

// ToDocument generates the step_N_valid.md content.
func (v *Verdict) ToDocument(stepNum int) string {
	var sb stringBuilder

	sb.WriteString("# Validation: Step " + itoa(stepNum) + "\n\n")

	// Build Status
	sb.WriteString("## Build Status\n")
	if v.BuildPassed {
		sb.WriteString("- Result: PASS\n")
	} else {
		sb.WriteString("- Result: FAIL\n")
	}
	sb.WriteString("\n")

	// Test Status
	sb.WriteString("## Test Status\n")
	if v.TestsPassed {
		sb.WriteString("- Result: PASS\n")
	} else {
		sb.WriteString("- Result: FAIL\n")
	}
	sb.WriteString("\n")

	// Requirements Verification
	sb.WriteString("## Requirements Verification\n")
	sb.WriteString("| REQ | Status | Evidence |\n")
	sb.WriteString("|-----|--------|----------|\n")
	for reqID, verified := range v.RequirementStatus {
		status := "✗"
		if verified {
			status = "✓"
		}
		sb.WriteString("| " + reqID + " | " + status + " | - |\n")
	}
	sb.WriteString("\n")

	// Acceptance Criteria
	sb.WriteString("## Acceptance Criteria\n")
	sb.WriteString("| AC | Status | Verification |\n")
	sb.WriteString("|----|--------|--------------|\n")
	for acID, verified := range v.AcceptanceStatus {
		status := "✗"
		if verified {
			status = "✓"
		}
		sb.WriteString("| " + acID + " | " + status + " | - |\n")
	}
	sb.WriteString("\n")

	// Cleanup Verification
	sb.WriteString("## Cleanup Verification\n")
	if v.CleanupVerified {
		sb.WriteString("- Status: VERIFIED\n")
	} else {
		sb.WriteString("- Status: NOT VERIFIED\n")
	}
	sb.WriteString("\n")

	// Style Compliance
	sb.WriteString("## Style Compliance\n")
	if v.StyleCompliant {
		sb.WriteString("- Status: COMPLIANT\n")
	} else {
		sb.WriteString("- Status: NON-COMPLIANT\n")
	}
	sb.WriteString("\n")

	// Verdict
	sb.WriteString("## Verdict: " + string(v.Status) + "\n\n")

	// Rejection Reasons
	if v.Status == VerdictReject && len(v.Reasons) > 0 {
		sb.WriteString("## Rejection Reasons\n")
		for i, reason := range v.Reasons {
			sb.WriteString(itoa(i+1) + ". " + reason + "\n")
		}
	}

	return sb.String()
}

// ParseVerdict parses a validation document into a Verdict.
func ParseVerdict(content string) (*Verdict, error) {
	verdict := &Verdict{
		Status:            VerdictPass,
		RequirementStatus: make(map[string]bool),
		AcceptanceStatus:  make(map[string]bool),
	}
	verdict.Document = content

	// Check for REJECT in verdict line
	if contains(content, "Verdict: reject") || contains(content, "Verdict: REJECT") {
		verdict.Status = VerdictReject
	}

	// Check build status
	if contains(content, "Build Status") {
		if contains(content, "Result: PASS") {
			verdict.BuildPassed = true
		}
	}

	// Check test status
	if contains(content, "Test Status") {
		if contains(content, "Result: PASS") {
			verdict.TestsPassed = true
		}
	}

	// Parse rejection reasons
	if verdict.Status == VerdictReject {
		sections := parseSections(content)
		if reasons, ok := sections["rejection reasons"]; ok {
			for _, line := range splitLines(reasons) {
				line = trimSpace(line)
				if len(line) > 2 && line[0] >= '1' && line[0] <= '9' && line[1] == '.' {
					verdict.Reasons = append(verdict.Reasons, trimSpace(line[2:]))
				}
			}
		}
	}

	return verdict, nil
}

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}

// AutoRejectConditions defines conditions for automatic rejection.
var AutoRejectConditions = []string{
	"Build fails",
	"Tests fail",
	"Requirements not traceable to code",
	"Dead code left behind",
	"Old function exists alongside replacement",
	"Codebase style violations",
	"Missing error handling",
	"Cleanup not performed as specified",
}

// CheckAutoReject checks if any auto-reject conditions are met.
func CheckAutoReject(result *StepResult, step *Step) *Verdict {
	if !result.BuildPassed {
		return Reject("Build failed").WithBuild(false)
	}

	if !result.TestsPassed {
		return Reject("Tests failed").WithTests(false)
	}

	// Check cleanup
	for _, item := range step.Cleanup {
		// In a real implementation, we would check if the item was actually removed
		_ = item
	}

	return nil // No auto-reject
}
