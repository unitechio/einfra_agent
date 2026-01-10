package package_executor

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"einfra/agent/internal/executor"
)

// Executor handles package management
type Executor struct{}

// NewExecutor creates a package executor
func NewExecutor() *Executor {
	return &Executor{}
}

// SupportedActions returns supported actions
func (e *Executor) SupportedActions() []string {
	return []string{
		"package_list",
		"package_install",
		"package_update",
	}
}

// Execute runs a package action
func (e *Executor) Execute(ctx context.Context, action *executor.Action) *executor.Result {
	result := &executor.Result{
		ActionID: action.ID,
		Data:     make(map[string]interface{}),
	}
	
	switch action.Type {
	case "package_list":
		return e.listPackages(ctx, result)
	case "package_install":
		return e.installPackage(ctx, action, result)
	default:
		result.Success = false
		result.Error = "unknown package action"
		return result
	}
}

// listPackages lists installed packages
func (e *Executor) listPackages(ctx context.Context, result *executor.Result) *executor.Result {
	var cmd *exec.Cmd
	
	if runtime.GOOS == "linux" {
		// Try dpkg first (Debian/Ubuntu)
		cmd = exec.CommandContext(ctx, "dpkg", "-l")
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			"Get-Package | Select-Object Name,Version | ConvertTo-Json")
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to list packages: %v", err)
		return result
	}
	
	result.Output = string(output)
	result.Success = true
	return result
}

// installPackage installs a package
func (e *Executor) installPackage(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	packageName, ok := action.Params["package"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'package' parameter"
		return result
	}
	
	var cmd *exec.Cmd
	
	if runtime.GOOS == "linux" {
		// Use apt-get (requires root)
		cmd = exec.CommandContext(ctx, "apt-get", "install", "-y", packageName)
	} else if runtime.GOOS == "windows" {
		// Use choco if available
		cmd = exec.CommandContext(ctx, "choco", "install", packageName, "-y")
	}
	
	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to install package: %v", err)
	} else {
		result.Success = true
	}
	
	return result
}
