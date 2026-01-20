package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Registry manages skill registration and lookup.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]sdk.Skill
	order  []string // Maintains registration order
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]sdk.Skill),
	}
}

// Register adds a skill to the registry.
func (r *Registry) Register(skill sdk.Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := skill.Metadata()
	if meta.Name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}

	if _, exists := r.skills[meta.Name]; exists {
		return fmt.Errorf("skill %q already registered", meta.Name)
	}

	r.skills[meta.Name] = skill
	r.order = append(r.order, meta.Name)
	return nil
}

// Unregister removes a skill from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[name]; !exists {
		return fmt.Errorf("skill %q not found", name)
	}

	delete(r.skills, name)

	// Remove from order slice
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}

	return nil
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) (sdk.Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[name]
	return skill, exists
}

// List returns all registered skills in registration order.
func (r *Registry) List() []sdk.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]sdk.Skill, len(r.order))
	for i, name := range r.order {
		skills[i] = r.skills[name]
	}
	return skills
}

// ListMetadata returns metadata for all registered skills.
func (r *Registry) ListMetadata() []sdk.SkillMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metas := make([]sdk.SkillMetadata, len(r.order))
	for i, name := range r.order {
		metas[i] = r.skills[name].Metadata()
	}
	return metas
}

// Count returns the number of registered skills.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}

// Clear removes all skills from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills = make(map[string]sdk.Skill)
	r.order = nil
}

// FindBest finds the skill with highest confidence for a task.
func (r *Registry) FindBest(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (sdk.Skill, float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var bestSkill sdk.Skill
	var bestConfidence float64

	for _, name := range r.order {
		skill := r.skills[name]
		canHandle, confidence := skill.CanHandle(ctx, execCtx, task)

		if canHandle && confidence > bestConfidence {
			bestSkill = skill
			bestConfidence = confidence
		}
	}

	return bestSkill, bestConfidence
}

// FindAll finds all skills that can handle a task.
func (r *Registry) FindAll(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) []SkillMatch {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []SkillMatch

	for _, name := range r.order {
		skill := r.skills[name]
		canHandle, confidence := skill.CanHandle(ctx, execCtx, task)

		if canHandle && confidence > 0 {
			matches = append(matches, SkillMatch{
				Skill:      skill,
				Confidence: confidence,
			})
		}
	}

	// Sort by confidence (descending)
	sortMatches(matches)

	return matches
}

// SkillMatch represents a skill with its confidence score.
type SkillMatch struct {
	Skill      sdk.Skill
	Confidence float64
}

// sortMatches sorts matches by confidence in descending order.
func sortMatches(matches []SkillMatch) {
	// Simple insertion sort (sufficient for small slices)
	for i := 1; i < len(matches); i++ {
		key := matches[i]
		j := i - 1
		for j >= 0 && matches[j].Confidence < key.Confidence {
			matches[j+1] = matches[j]
			j--
		}
		matches[j+1] = key
	}
}

// FindByTrigger finds skills matching a trigger pattern.
func (r *Registry) FindByTrigger(text string) []sdk.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []sdk.Skill

	for _, name := range r.order {
		skill := r.skills[name]
		meta := skill.Metadata()

		if sdk.MatchTrigger(text, meta.Triggers) {
			matched = append(matched, skill)
		}
	}

	return matched
}

// FindByTag finds skills with the specified tag.
func (r *Registry) FindByTag(tag string) []sdk.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []sdk.Skill

	for _, name := range r.order {
		skill := r.skills[name]
		meta := skill.Metadata()

		for _, t := range meta.Tags {
			if t == tag {
				matched = append(matched, skill)
				break
			}
		}
	}

	return matched
}

// HasSkill checks if a skill is registered.
func (r *Registry) HasSkill(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.skills[name]
	return exists
}

// Names returns the names of all registered skills.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.order))
	copy(names, r.order)
	return names
}
