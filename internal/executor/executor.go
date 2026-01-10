package executor

import (
	"context"
	"fmt"
)

// Action represents a task to execute
type Action struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Params  map[string]interface{} `json:"params"`
	Timeout int                    `json:"timeout"` // seconds
}

// Result is the outcome of an action
type Result struct {
	ActionID string                 `json:"action_id"`
	Success  bool                   `json:"success"`
	Output   string                 `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// Executor interface for all action handlers
type Executor interface {
	Execute(ctx context.Context, action *Action) *Result
	SupportedActions() []string
}

// Registry maps action types to executors
type Registry struct {
	executors map[string]Executor
}

// NewRegistry creates a new executor registry
func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]Executor),
	}
}

// Register adds an executor to the registry
func (r *Registry) Register(exec Executor) {
	for _, actionType := range exec.SupportedActions() {
		r.executors[actionType] = exec
	}
}

// Execute runs an action
func (r *Registry) Execute(ctx context.Context, action *Action) *Result {
	exec, ok := r.executors[action.Type]
	if !ok {
		return &Result{
			ActionID: action.ID,
			Success:  false,
			Error:    fmt.Sprintf("unsupported action type: %s", action.Type),
		}
	}

	return exec.Execute(ctx, action)
}
