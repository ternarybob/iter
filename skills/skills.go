// Package skills provides default skills for common DevOps tasks.
package skills

import (
	"github.com/ternarybob/iter/pkg/sdk"
	"github.com/ternarybob/iter/skills/codemod"
	"github.com/ternarybob/iter/skills/devops"
	"github.com/ternarybob/iter/skills/docs"
	"github.com/ternarybob/iter/skills/patch"
	"github.com/ternarybob/iter/skills/review"
	"github.com/ternarybob/iter/skills/test"
)

// All returns all default skills.
func All() []sdk.Skill {
	return []sdk.Skill{
		codemod.New(),
		test.New(),
		review.New(),
		patch.New(),
		devops.New(),
		docs.New(),
	}
}

// Codemod returns the code modification skill.
func Codemod() sdk.Skill {
	return codemod.New()
}

// Test returns the test generation/execution skill.
func Test() sdk.Skill {
	return test.New()
}

// Review returns the code review skill.
func Review() sdk.Skill {
	return review.New()
}

// Patch returns the patch application skill.
func Patch() sdk.Skill {
	return patch.New()
}

// DevOps returns the DevOps skill.
func DevOps() sdk.Skill {
	return devops.New()
}

// Docs returns the documentation skill.
func Docs() sdk.Skill {
	return docs.New()
}
