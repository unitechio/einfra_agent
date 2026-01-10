package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"einfra/agent/internal/executor"
	"einfra/agent/internal/logger"
)

// Executor handles service management actions
type Executor struct{}

// NewExecutor creates a service executor
func NewExecutor() *Executor {
	return &Executor{}
}

// SupportedActions returns list of supported action types
func (e *Executor) SupportedActions() []string {
	return []string{
		"service_list",
		"service_start",
		"service_stop",
		"service_restart",
		"service_reload",
		"service_enable",
		"service_disable",
		"service_status",
	}
}

// Execute runs a service action
func (e *Executor) Execute(ctx context.Context, action *executor.Action) *executor.Result {
	result := &executor.Result{
		ActionID: action.ID,
		Data:     make(map[string]interface{}),
	}

	switch action.Type {
	case "service_list":
		return e.listServices(ctx, result)
	case "service_start", "service_stop", "service_restart", "service_reload":
		return e.controlService(ctx, action, result)
	case "service_enable", "service_disable":
		return e.bootControl(ctx, action, result)
	case "service_status":
		return e.getStatus(ctx, action, result)
	default:
		result.Success = false
		result.Error = "unknown service action"
		return result
	}
}

// listServices lists all services
func (e *Executor) listServices(ctx context.Context, result *executor.Result) *executor.Result {
	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "systemctl", "list-units", "--type=service", "--all", "--no-pager", "--output=json")
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command", "Get-Service | ConvertTo-Json")
	} else {
		result.Success = false
		result.Error = "unsupported platform"
		return result
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to list services: %v", err)
		return result
	}

	// Parse output
	var services []map[string]interface{}
	if err := json.Unmarshal(output, &services); err != nil {
		// Fallback to text parsing if JSON fails
		result.Output = string(output)
	} else {
		result.Data["services"] = services
	}

	result.Success = true
	return result
}

// controlService starts/stops/restarts a service
func (e *Executor) controlService(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	serviceName, ok := action.Params["service"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'service' parameter"
		return result
	}

	var cmd *exec.Cmd
	actionType := strings.TrimPrefix(action.Type, "service_")

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "systemctl", actionType, serviceName)
	} else if runtime.GOOS == "windows" {
		var psAction string
		switch actionType {
		case "start":
			psAction = "Start-Service"
		case "stop":
			psAction = "Stop-Service"
		case "restart":
			psAction = "Restart-Service"
		default:
			result.Success = false
			result.Error = "unsupported action on Windows"
			return result
		}
		cmd = exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf("%s -Name %s", psAction, serviceName))
	}

	logger.Info().
		Str("action", action.Type).
		Str("service", serviceName).
		Msg("Executing service action")

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("command failed: %v", err)
		logger.Error().
			Err(err).
			Str("service", serviceName).
			Str("output", result.Output).
			Msg("Service action failed")
	} else {
		result.Success = true
		logger.Info().
			Str("service", serviceName).
			Msg("Service action completed successfully")
	}

	return result
}

// bootControl enables/disables service at boot
func (e *Executor) bootControl(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	serviceName, ok := action.Params["service"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'service' parameter"
		return result
	}

	var cmd *exec.Cmd
	actionType := strings.TrimPrefix(action.Type, "service_")

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "systemctl", actionType, serviceName)
	} else if runtime.GOOS == "windows" {
		var startType string
		if actionType == "enable" {
			startType = "Automatic"
		} else {
			startType = "Disabled"
		}
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			fmt.Sprintf("Set-Service -Name %s -StartupType %s", serviceName, startType))
	}

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("command failed: %v", err)
	} else {
		result.Success = true
	}

	return result
}

// getStatus gets service status
func (e *Executor) getStatus(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	serviceName, ok := action.Params["service"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'service' parameter"
		return result
	}

	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "systemctl", "status", serviceName, "--no-pager")
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			fmt.Sprintf("Get-Service -Name %s | ConvertTo-Json", serviceName))
	}

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		// Status command returns non-zero for inactive services, which is OK
		result.Success = true
	} else {
		result.Success = true
	}

	return result
}
