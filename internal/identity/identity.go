package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

// Identity represents the agent's unique identity
type Identity struct {
	NodeID      string
	Fingerprint string
	Hostname    string
	Platform    string
	Arch        string
}

// Generate creates or loads the agent identity
func Generate() (*Identity, error) {
	hostname, _ := os.Hostname()

	id := &Identity{
		Hostname: hostname,
		Platform: runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	// Generate NodeID (UUID v4)
	id.NodeID = uuid.New().String()

	// Generate Fingerprint (platform-specific)
	fp, err := generateFingerprint()
	if err != nil {
		return nil, fmt.Errorf("failed to generate fingerprint: %w", err)
	}
	id.Fingerprint = fp

	return id, nil
}

// generateFingerprint creates a stable fingerprint based on hardware
func generateFingerprint() (string, error) {
	var components []string

	if runtime.GOOS == "linux" {
		// Read machine-id
		machineID, err := os.ReadFile("/etc/machine-id")
		if err == nil {
			components = append(components, strings.TrimSpace(string(machineID)))
		}
	} else if runtime.GOOS == "windows" {
		// Use MachineGuid from registry (simplified - in production use syscall)
		// For now, use hostname as fallback
		hostname, _ := os.Hostname()
		components = append(components, hostname)
	}

	// Add MAC address
	mac, err := getMACAddress()
	if err == nil {
		components = append(components, mac)
	}

	// Hash all components
	hash := sha256.New()
	hash.Write([]byte(strings.Join(components, "|")))
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getMACAddress returns the first non-loopback MAC address
func getMACAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == 0 && iface.HardwareAddr != nil {
			return iface.HardwareAddr.String(), nil
		}
	}

	return "", fmt.Errorf("no MAC address found")
}
