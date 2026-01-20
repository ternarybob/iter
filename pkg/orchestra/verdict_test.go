package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerdict_Pass(t *testing.T) {
	verdict := Pass()

	assert.Equal(t, VerdictPass, verdict.Status)
	assert.True(t, verdict.IsPassing())
	assert.False(t, verdict.IsRejecting())
}

func TestVerdict_Reject(t *testing.T) {
	verdict := Reject("requirement not met", "tests failing")

	assert.Equal(t, VerdictReject, verdict.Status)
	assert.False(t, verdict.IsPassing())
	assert.True(t, verdict.IsRejecting())
	assert.Len(t, verdict.Reasons, 2)
	assert.Contains(t, verdict.Reasons, "requirement not met")
	assert.Contains(t, verdict.Reasons, "tests failing")
}

func TestVerdict_SetRequirement(t *testing.T) {
	verdict := Pass().
		SetRequirement("req-1", true).
		SetRequirement("req-2", true).
		SetRequirement("req-3", false)

	assert.True(t, verdict.RequirementStatus["req-1"])
	assert.True(t, verdict.RequirementStatus["req-2"])
	assert.False(t, verdict.RequirementStatus["req-3"])
}

func TestVerdict_WithBuild(t *testing.T) {
	verdict := Pass().WithBuild(true).WithTests(true)

	assert.True(t, verdict.BuildPassed)
	assert.True(t, verdict.TestsPassed)
}

func TestVerdict_WithCleanup(t *testing.T) {
	verdict := Pass().WithCleanup(true)

	assert.True(t, verdict.CleanupVerified)
}

func TestVerdict_WithReason(t *testing.T) {
	verdict := Reject("initial reason").WithReason("additional reason")

	assert.Len(t, verdict.Reasons, 2)
	assert.Contains(t, verdict.Reasons, "initial reason")
	assert.Contains(t, verdict.Reasons, "additional reason")
}

func TestVerdict_ToDocument(t *testing.T) {
	verdict := Reject("test failure", "missing cleanup")

	doc := verdict.ToDocument(1)

	assert.Contains(t, doc, "reject")
	assert.Contains(t, doc, "Step 1")
}

func TestVerdict_PassToDocument(t *testing.T) {
	verdict := Pass()

	doc := verdict.ToDocument(1)

	assert.Contains(t, doc, "pass")
}

func TestVerdict_AllRequirementsMet(t *testing.T) {
	verdict := Pass().
		SetRequirement("r1", true).
		SetRequirement("r2", true)
	
	assert.True(t, verdict.AllRequirementsMet())

	verdict.SetRequirement("r3", false)
	assert.False(t, verdict.AllRequirementsMet())
}

func TestVerdict_WithBuildFails(t *testing.T) {
	// If build fails, verdict should become reject
	verdict := Pass().WithBuild(false)
	
	assert.Equal(t, VerdictReject, verdict.Status)
	assert.Contains(t, verdict.Reasons, "Build failed")
}

func TestVerdict_WithTestsFails(t *testing.T) {
	verdict := Pass().WithTests(false)
	
	assert.Equal(t, VerdictReject, verdict.Status)
	assert.Contains(t, verdict.Reasons, "Tests failed")
}

func TestVerdict_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		status        VerdictStatus
		reasons       []string
		buildPassed   bool
		testsPassed   bool
		cleanupOK     bool
		requirements  map[string]bool
		wantPassing   bool
		wantReasonCnt int
	}{
		{
			name:          "clean pass",
			status:        VerdictPass,
			reasons:       []string{},
			buildPassed:   true,
			testsPassed:   true,
			cleanupOK:     true,
			requirements:  map[string]bool{"r1": true},
			wantPassing:   true,
			wantReasonCnt: 0,
		},
		{
			name:          "reject with reasons",
			status:        VerdictReject,
			reasons:       []string{"reason1", "reason2"},
			buildPassed:   false,
			testsPassed:   false,
			cleanupOK:     false,
			requirements:  map[string]bool{"r1": false},
			wantPassing:   false,
			wantReasonCnt: 2,
		},
		{
			name:          "reject no reasons",
			status:        VerdictReject,
			reasons:       []string{},
			buildPassed:   true,
			testsPassed:   true,
			cleanupOK:     true,
			requirements:  map[string]bool{},
			wantPassing:   false,
			wantReasonCnt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict := &Verdict{
				Status:            tt.status,
				Reasons:           tt.reasons,
				BuildPassed:       tt.buildPassed,
				TestsPassed:       tt.testsPassed,
				CleanupVerified:   tt.cleanupOK,
				RequirementStatus: tt.requirements,
			}

			assert.Equal(t, tt.wantPassing, verdict.IsPassing())
			assert.Len(t, verdict.Reasons, tt.wantReasonCnt)
		})
	}
}

func TestVerdictStatus_String(t *testing.T) {
	assert.Equal(t, "pass", string(VerdictPass))
	assert.Equal(t, "reject", string(VerdictReject))
}

func TestParseVerdict(t *testing.T) {
	content := `# Validation: Step 1

## Build Status
- Result: PASS

## Verdict: pass
`
	verdict, err := ParseVerdict(content)
	assert.NoError(t, err)
	assert.True(t, verdict.IsPassing())
}

func TestParseVerdict_Reject(t *testing.T) {
	content := `# Validation: Step 1

## Verdict: reject

## Rejection Reasons
1. Build failed
2. Tests not passing
`
	verdict, err := ParseVerdict(content)
	assert.NoError(t, err)
	assert.True(t, verdict.IsRejecting())
}
