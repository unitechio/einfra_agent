package user

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"einfra/agent/internal/executor"
)

// Executor handles user management
type Executor struct{}

// NewExecutor creates a user executor
func NewExecutor() *Executor {
	return &Executor{}
}

// SupportedActions returns supported actions
func (e *Executor) SupportedActions() []string {
	return []string{
		"user_list",
		"user_add",
		"user_delete",
		"group_list",
	}
}

// Execute runs a user action
func (e *Executor) Execute(ctx context.Context, action *executor.Action) *executor.Result {
	result := &executor.Result{
		ActionID: action.ID,
		Data:     make(map[string]interface{}),
	}

	switch action.Type {
	case "user_list":
		return e.listUsers(ctx, result)
	case "user_add":
		return e.addUser(ctx, action, result)
	case "user_delete":
		return e.deleteUser(ctx, action, result)
	case "group_list":
		return e.listGroups(ctx, result)
	default:
		result.Success = false
		result.Error = "unknown user action"
		return result
	}
}

// listUsers lists all users
func (e *Executor) listUsers(ctx context.Context, result *executor.Result) *executor.Result {
	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "getent", "passwd")
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			"Get-LocalUser | ConvertTo-Json")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to list users: %v", err)
		return result
	}

	result.Output = string(output)
	result.Success = true
	return result
}

// addUser creates a new user
func (e *Executor) addUser(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	username, ok := action.Params["username"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'username' parameter"
		return result
	}

	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "useradd", username)
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			fmt.Sprintf("New-LocalUser -Name %s -NoPassword", username))
	}

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to add user: %v", err)
	} else {
		result.Success = true
	}

	return result
}

// deleteUser removes a user
func (e *Executor) deleteUser(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	username, ok := action.Params["username"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'username' parameter"
		return result
	}

	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "userdel", username)
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			fmt.Sprintf("Remove-LocalUser -Name %s", username))
	}

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to delete user: %v", err)
	} else {
		result.Success = true
	}

	return result
}

// listGroups lists all groups
func (e *Executor) listGroups(ctx context.Context, result *executor.Result) *executor.Result {
	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "getent", "group")
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			"Get-LocalGroup | ConvertTo-Json")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to list groups: %v", err)
		return result
	}

	// Parse groups
	groups := parseGroups(string(output))
	result.Data["groups"] = groups
	result.Success = true
	return result
}

// parseGroups parses group output
func parseGroups(output string) []map[string]interface{} {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	groups := make([]map[string]interface{}, 0, len(lines))

	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 {
			groups = append(groups, map[string]interface{}{
				"name": parts[0],
				"gid":  parts[2],
			})
		}
	}

	return groups
}
