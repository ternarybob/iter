package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements Provider for testing
type mockProvider struct {
	name   string
	models []string
	resp   *CompletionResponse
	err    error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Models() []string {
	return m.models
}

func (m *mockProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &CompletionResponse{
		ID:           "test-id",
		Model:        req.Model,
		Content:      "test response",
		FinishReason: "stop",
	}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Content: "test", Done: true}
	close(ch)
	return ch, nil
}

func (m *mockProvider) CountTokens(content string) (int, error) {
	return len(content) / 4, nil // rough estimate
}

func TestRouter_Creation(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"model-a", "model-b"},
	}

	router := NewRouter(provider)

	assert.NotNil(t, router)
	assert.Equal(t, "router:test", router.Name())
	assert.Equal(t, []string{"model-a", "model-b"}, router.Models())
}

func TestRouter_SetModels(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"default"},
	}

	router := NewRouter(provider)

	router.SetPlanningModel("planning-model")
	router.SetExecutionModel("execution-model")
	router.SetValidationModel("validation-model")

	assert.Equal(t, "planning-model", router.PlanningModel())
	assert.Equal(t, "execution-model", router.ExecutionModel())
	assert.Equal(t, "validation-model", router.ValidationModel())
}

func TestRouter_Complete(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"model-a"},
		resp: &CompletionResponse{
			ID:           "resp-1",
			Model:        "model-a",
			Content:      "Hello, world!",
			FinishReason: "stop",
		},
	}

	router := NewRouter(provider)
	ctx := context.Background()

	resp, err := router.Complete(ctx, &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", resp.Content)
}

func TestRouter_ForPlanning(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"model-a"},
	}

	router := NewRouter(provider)
	router.SetPlanningModel("opus")

	planningProvider := router.ForPlanning()

	assert.NotNil(t, planningProvider)
	assert.Equal(t, []string{"opus"}, planningProvider.Models())
}

func TestRouter_ForExecution(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"model-a"},
	}

	router := NewRouter(provider)
	router.SetExecutionModel("sonnet")

	execProvider := router.ForExecution()

	assert.NotNil(t, execProvider)
	assert.Equal(t, []string{"sonnet"}, execProvider.Models())
}

func TestRouter_ForValidation(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"model-a"},
	}

	router := NewRouter(provider)
	router.SetValidationModel("opus")

	validProvider := router.ForValidation()

	assert.NotNil(t, validProvider)
	assert.Equal(t, []string{"opus"}, validProvider.Models())
}

func TestRouter_CountTokens(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"model-a"},
	}

	router := NewRouter(provider)

	count, err := router.CountTokens("hello world")
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestRouter_Stream(t *testing.T) {
	provider := &mockProvider{
		name:   "test",
		models: []string{"model-a"},
	}

	router := NewRouter(provider)
	ctx := context.Background()

	ch, err := router.Stream(ctx, &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	require.NoError(t, err)

	var content string
	for chunk := range ch {
		content += chunk.Content
		if chunk.Done {
			break
		}
	}

	assert.NotEmpty(t, content)
}

func TestMultiProvider_Creation(t *testing.T) {
	p1 := &mockProvider{name: "p1", models: []string{"m1"}}
	p2 := &mockProvider{name: "p2", models: []string{"m2"}}

	mp := NewMultiProvider(p1, p2)

	assert.Equal(t, "multi:p1", mp.Name())
	assert.Contains(t, mp.Models(), "m1")
	assert.Contains(t, mp.Models(), "m2")
}

func TestMultiProvider_SetPrimary(t *testing.T) {
	p1 := &mockProvider{name: "p1", models: []string{"m1"}}
	p2 := &mockProvider{name: "p2", models: []string{"m2"}}

	mp := NewMultiProvider(p1, p2)

	err := mp.SetPrimary(1)
	require.NoError(t, err)
	assert.Equal(t, "multi:p2", mp.Name())

	err = mp.SetPrimary(5) // invalid
	assert.Error(t, err)
}

func TestMultiProvider_Complete(t *testing.T) {
	p1 := &mockProvider{
		name:   "p1",
		models: []string{"m1"},
		resp:   &CompletionResponse{Content: "from p1"},
	}

	mp := NewMultiProvider(p1)
	ctx := context.Background()

	resp, err := mp.Complete(ctx, &CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "from p1", resp.Content)
}

func TestRouter_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		planningModel string
		execModel     string
		validModel    string
		wantPlanning  string
		wantExec      string
		wantValid     string
	}{
		{
			name:          "all different",
			planningModel: "opus",
			execModel:     "sonnet",
			validModel:    "haiku",
			wantPlanning:  "opus",
			wantExec:      "sonnet",
			wantValid:     "haiku",
		},
		{
			name:          "all same",
			planningModel: "opus",
			execModel:     "opus",
			validModel:    "opus",
			wantPlanning:  "opus",
			wantExec:      "opus",
			wantValid:     "opus",
		},
		{
			name:          "planning and validation same",
			planningModel: "opus",
			execModel:     "sonnet",
			validModel:    "opus",
			wantPlanning:  "opus",
			wantExec:      "sonnet",
			wantValid:     "opus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockProvider{
				name:   "test",
				models: []string{"opus", "sonnet", "haiku"},
			}

			router := NewRouter(provider)
			router.SetPlanningModel(tt.planningModel)
			router.SetExecutionModel(tt.execModel)
			router.SetValidationModel(tt.validModel)

			assert.Equal(t, tt.wantPlanning, router.PlanningModel())
			assert.Equal(t, tt.wantExec, router.ExecutionModel())
			assert.Equal(t, tt.wantValid, router.ValidationModel())
		})
	}
}
