package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"einfra/agent/internal/executor"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// Executor handles system information and monitoring
type Executor struct{}

// NewExecutor creates a system executor
func NewExecutor() *Executor {
	return &Executor{}
}

// SupportedActions returns supported actions
func (e *Executor) SupportedActions() []string {
	return []string{
		"system_info",
		"system_metrics",
		"process_list",
	}
}

// Execute runs a system action
func (e *Executor) Execute(ctx context.Context, action *executor.Action) *executor.Result {
	result := &executor.Result{
		ActionID: action.ID,
		Data:     make(map[string]interface{}),
	}

	switch action.Type {
	case "system_info":
		return e.getSystemInfo(ctx, result)
	case "system_metrics":
		return e.getMetrics(ctx, result)
	case "process_list":
		return e.getProcesses(ctx, result)
	default:
		result.Success = false
		result.Error = "unknown system action"
		return result
	}
}

// getSystemInfo retrieves system information
func (e *Executor) getSystemInfo(ctx context.Context, result *executor.Result) *executor.Result {
	hostname, _ := os.Hostname()
	hostInfo, _ := host.Info()

	result.Data["hostname"] = hostname
	result.Data["os"] = hostInfo.OS
	result.Data["platform"] = hostInfo.Platform
	result.Data["platform_version"] = hostInfo.PlatformVersion
	result.Data["kernel"] = hostInfo.KernelVersion
	result.Data["arch"] = runtime.GOARCH
	result.Data["uptime"] = hostInfo.Uptime

	result.Success = true
	return result
}

// getMetrics collects system metrics
func (e *Executor) getMetrics(ctx context.Context, result *executor.Result) *executor.Result {
	// CPU
	cpuPercent, _ := cpu.Percent(0, false)
	if len(cpuPercent) > 0 {
		result.Data["cpu_percent"] = cpuPercent[0]
	}

	// Memory
	memInfo, _ := mem.VirtualMemory()
	result.Data["memory_total"] = memInfo.Total
	result.Data["memory_used"] = memInfo.Used
	result.Data["memory_percent"] = memInfo.UsedPercent

	// Disk
	diskInfo, _ := disk.Usage("/")
	result.Data["disk_total"] = diskInfo.Total
	result.Data["disk_used"] = diskInfo.Used
	result.Data["disk_percent"] = diskInfo.UsedPercent

	// Network
	netIO, _ := net.IOCounters(false)
	if len(netIO) > 0 {
		result.Data["net_bytes_sent"] = netIO[0].BytesSent
		result.Data["net_bytes_recv"] = netIO[0].BytesRecv
	}

	result.Success = true
	return result
}

// getProcesses lists running processes
func (e *Executor) getProcesses(ctx context.Context, result *executor.Result) *executor.Result {
	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "ps", "aux", "--sort=-%cpu", "--no-headers")
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			"Get-Process | Sort-Object CPU -Descending | Select-Object -First 50 | ConvertTo-Json")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to get processes: %v", err)
		return result
	}

	// Parse process list
	processes := parseProcessList(string(output))
	result.Data["processes"] = processes
	result.Success = true
	return result
}

// parseProcessList parses ps output into structured data
func parseProcessList(output string) []map[string]interface{} {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	processes := make([]map[string]interface{}, 0, len(lines))

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 64)

		processes = append(processes, map[string]interface{}{
			"user":    fields[0],
			"pid":     fields[1],
			"cpu":     cpu,
			"mem":     mem,
			"command": strings.Join(fields[10:], " "),
		})
	}

	return processes
}
