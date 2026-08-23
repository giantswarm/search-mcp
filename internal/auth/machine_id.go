package auth

import (
	"crypto/sha256"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// osWindows is the runtime.GOOS value for Windows.
const osWindows = "windows"

// DeriveEncryptionKey generates a 32-byte encryption key from machine identifiers
// This provides a balance between security and usability - the key is stable
// across restarts but unique per machine.
func DeriveEncryptionKey() ([]byte, error) {
	var identifiers []string

	switch runtime.GOOS {
	case "darwin":
		// macOS: try to get hardware UUID
		identifiers = append(identifiers, getMacOSMachineID()...)

	case "linux":
		// Linux: use machine-id
		identifiers = append(identifiers, getLinuxMachineID()...)

	case osWindows:
		// Windows: use hostname as primary identifier
		// Note: A more robust solution would use Windows registry MachineGUID
		identifiers = append(identifiers, getWindowsMachineID()...)
	}

	// Always add hostname as fallback/additional entropy
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		identifiers = append(identifiers, hostname)
	}

	if len(identifiers) == 0 {
		return nil, fmt.Errorf("failed to derive encryption key: no machine identifiers found")
	}

	// Combine all identifiers with separator
	combined := strings.Join(identifiers, "|")

	// Hash to get 32-byte key for AES-256
	hash := sha256.Sum256([]byte(combined))
	return hash[:], nil
}

// getMacOSMachineID attempts to get macOS hardware identifiers
func getMacOSMachineID() []string {
	var ids []string

	// Try IOPlatformUUID (most reliable on macOS)
	if data, err := os.ReadFile("/var/db/.MachineGUID"); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// getLinuxMachineID attempts to get Linux machine identifiers
func getLinuxMachineID() []string {
	var ids []string

	// Try /etc/machine-id (systemd)
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			ids = append(ids, id)
		}
	}

	// Try /var/lib/dbus/machine-id (D-Bus)
	if len(ids) == 0 {
		if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				ids = append(ids, id)
			}
		}
	}

	// Try product UUID
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// getWindowsMachineID attempts to get Windows machine identifiers
func getWindowsMachineID() []string {
	var ids []string

	// On Windows, we primarily rely on hostname
	// A production implementation could use:
	// - Windows Registry: HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography\MachineGuid
	// - Windows Management Instrumentation (WMI)
	// For now, hostname provides reasonable uniqueness

	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		ids = append(ids, hostname)
	}

	return ids
}
