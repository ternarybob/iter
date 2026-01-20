package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ternarybob/iter/pkg/sdk"
)

func TestAll_ReturnsSkills(t *testing.T) {
	skills := All()

	assert.NotEmpty(t, skills, "should return skills")

	// Check that we have the expected default skills
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Metadata().Name
	}

	assert.Contains(t, names, "codemod", "should have codemod skill")
	assert.Contains(t, names, "test", "should have test skill")
	assert.Contains(t, names, "review", "should have review skill")
	assert.Contains(t, names, "patch", "should have patch skill")
	assert.Contains(t, names, "devops", "should have devops skill")
	assert.Contains(t, names, "docs", "should have docs skill")
}

func TestAll_SkillsHaveMetadata(t *testing.T) {
	skills := All()

	for _, skill := range skills {
		meta := skill.Metadata()

		assert.NotEmpty(t, meta.Name, "skill should have name")
		assert.NotEmpty(t, meta.Description, "skill should have description")
		assert.NotEmpty(t, meta.Triggers, "skill should have triggers")
	}
}

func TestAll_SkillsHaveTriggers(t *testing.T) {
	skills := All()

	for _, skill := range skills {
		meta := skill.Metadata()
		t.Run(meta.Name, func(t *testing.T) {
			assert.NotEmpty(t, meta.Triggers, "skill %s should have triggers", meta.Name)

			// Triggers should be lowercase
			for _, trigger := range meta.Triggers {
				assert.NotEmpty(t, trigger, "trigger should not be empty")
			}
		})
	}
}

func TestAll_NoDuplicateNames(t *testing.T) {
	skills := All()

	seen := make(map[string]bool)
	for _, skill := range skills {
		name := skill.Metadata().Name
		assert.False(t, seen[name], "duplicate skill name: %s", name)
		seen[name] = true
	}
}

func TestAll_SkillCount(t *testing.T) {
	skills := All()
	
	// Should have exactly 6 default skills
	assert.Len(t, skills, 6)
}

func TestAll_TableDriven(t *testing.T) {
	tests := []struct {
		skillName   string
		wantTrigger string
		wantDesc    bool
	}{
		{
			skillName:   "codemod",
			wantTrigger: "fix",
			wantDesc:    true,
		},
		{
			skillName:   "test",
			wantTrigger: "test",
			wantDesc:    true,
		},
		{
			skillName:   "review",
			wantTrigger: "review",
			wantDesc:    true,
		},
		{
			skillName:   "patch",
			wantTrigger: "patch",
			wantDesc:    true,
		},
		{
			skillName:   "devops",
			wantTrigger: "docker",
			wantDesc:    true,
		},
		{
			skillName:   "docs",
			wantTrigger: "document",
			wantDesc:    true,
		},
	}

	skills := All()
	skillMap := make(map[string]sdk.Skill)
	for _, s := range skills {
		skillMap[s.Metadata().Name] = s
	}

	for _, tt := range tests {
		t.Run(tt.skillName, func(t *testing.T) {
			skill, exists := skillMap[tt.skillName]
			require.True(t, exists, "skill %s should exist", tt.skillName)

			meta := skill.Metadata()

			// Check trigger exists
			hasTrigger := false
			for _, trigger := range meta.Triggers {
				if trigger == tt.wantTrigger {
					hasTrigger = true
					break
				}
			}
			assert.True(t, hasTrigger, "skill %s should have trigger %s", tt.skillName, tt.wantTrigger)

			if tt.wantDesc {
				assert.NotEmpty(t, meta.Description, "skill %s should have description", tt.skillName)
			}
		})
	}
}
