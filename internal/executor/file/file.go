package file

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"einfra/agent/internal/executor"
)

// Executor handles file operations
type Executor struct{}

// NewExecutor creates a file executor
func NewExecutor() *Executor {
	return &Executor{}
}

// SupportedActions returns supported actions
func (e *Executor) SupportedActions() []string {
	return []string{
		"file_list",
		"file_read",
		"file_write",
		"file_delete",
		"file_chmod",
		"file_chown",
		"dir_create",
	}
}

// Execute runs a file action
func (e *Executor) Execute(ctx context.Context, action *executor.Action) *executor.Result {
	result := &executor.Result{
		ActionID: action.ID,
		Data:     make(map[string]interface{}),
	}

	switch action.Type {
	case "file_list":
		return e.listFiles(ctx, action, result)
	case "file_read":
		return e.readFile(ctx, action, result)
	case "file_delete":
		return e.deleteFile(ctx, action, result)
	case "file_chmod":
		return e.chmod(ctx, action, result)
	case "dir_create":
		return e.createDir(ctx, action, result)
	default:
		result.Success = false
		result.Error = "unknown file action"
		return result
	}
}

// listFiles lists directory contents
func (e *Executor) listFiles(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	path, ok := action.Params["path"].(string)
	if !ok {
		path = "/"
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to read directory: %v", err)
		return result
	}

	files := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		files = append(files, map[string]interface{}{
			"name":   entry.Name(),
			"is_dir": entry.IsDir(),
			"size":   info.Size(),
			"mode":   info.Mode().String(),
		})
	}

	result.Data["files"] = files
	result.Success = true
	return result
}

// readFile reads file content (limited size)
func (e *Executor) readFile(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	path, ok := action.Params["path"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'path' parameter"
		return result
	}

	// Limit read size to 1MB
	const maxSize = 1024 * 1024

	info, err := os.Stat(path)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("file not found: %v", err)
		return result
	}

	if info.Size() > maxSize {
		result.Success = false
		result.Error = "file too large (max 1MB)"
		return result
	}

	content, err := os.ReadFile(path)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to read file: %v", err)
		return result
	}

	result.Data["content"] = string(content)
	result.Success = true
	return result
}

// deleteFile deletes a file
func (e *Executor) deleteFile(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	path, ok := action.Params["path"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'path' parameter"
		return result
	}

	if err := os.Remove(path); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to delete file: %v", err)
	} else {
		result.Success = true
	}

	return result
}

// chmod changes file permissions (Linux only)
func (e *Executor) chmod(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	if runtime.GOOS != "linux" {
		result.Success = false
		result.Error = "chmod only supported on Linux"
		return result
	}

	path, ok := action.Params["path"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'path' parameter"
		return result
	}

	mode, ok := action.Params["mode"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'mode' parameter"
		return result
	}

	cmd := exec.CommandContext(ctx, "chmod", mode, path)
	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("chmod failed: %v", err)
	} else {
		result.Success = true
	}

	return result
}

// createDir creates a directory
func (e *Executor) createDir(ctx context.Context, action *executor.Action, result *executor.Result) *executor.Result {
	path, ok := action.Params["path"].(string)
	if !ok {
		result.Success = false
		result.Error = "missing 'path' parameter"
		return result
	}

	if err := os.MkdirAll(filepath.Clean(path), 0755); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create directory: %v", err)
	} else {
		result.Success = true
	}

	return result
}
