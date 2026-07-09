//go:build darwin

package printer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RFCOMMConnection is a compatibility type for macOS.
// On macOS, paired Bluetooth SPP devices appear as /dev/cu.* serial ports
// automatically, so there is no RFCOMM/COM setup to manage.
type RFCOMMConnection struct {
	DevicePath string
	MAC        string
}

// ListPairedBluetoothDevices returns Bluetooth serial ports on macOS.
// Paired BT SPP devices show up as /dev/cu.* entries; we surface those.
func ListPairedBluetoothDevices() ([]BluetoothDevice, error) {
	var devices []BluetoothDevice

	ports, err := ListSerialPorts()
	if err != nil {
		return nil, err
	}

	for _, port := range ports {
		// Skip the built-in incoming-port / debug entries that aren't printers.
		base := strings.ToLower(filepath.Base(port))
		if strings.Contains(base, "incoming-port") || strings.Contains(base, "debug-console") {
			continue
		}
		devices = append(devices, BluetoothDevice{
			Name: filepath.Base(port),
			MAC:  port, // On macOS, the device path is the identifier.
		})
	}

	return devices, nil
}

// CheckRFCOMMInstalled always returns nil on macOS (not needed).
func CheckRFCOMMInstalled() error {
	return nil
}

// CheckPrivilegeHelper returns "darwin" on macOS (no elevation needed for /dev/cu.*).
func CheckPrivilegeHelper() string {
	return "darwin"
}

// EstablishRFCOMM on macOS simply returns the serial device path.
// macOS exposes BT SPP as a /dev/cu.* port, so no special setup is required.
func EstablishRFCOMM(mac string, channel int, statusCallback func(string)) (*RFCOMMConnection, error) {
	// On macOS, 'mac' is actually the device path (e.g., /dev/cu.NelkoP21).
	if statusCallback != nil {
		statusCallback(fmt.Sprintf("Using port %s...", mac))
	}

	if !strings.HasPrefix(mac, "/dev/") {
		return nil, fmt.Errorf("invalid serial device: %s", mac)
	}

	if _, err := os.Stat(mac); err != nil {
		return nil, fmt.Errorf("%w: %s not found", ErrNoDevicesFound, mac)
	}

	conn := &RFCOMMConnection{
		DevicePath: mac,
		MAC:        mac,
	}

	if statusCallback != nil {
		statusCallback(fmt.Sprintf("Ready: %s", mac))
	}

	return conn, nil
}

// Close is a no-op on macOS (serial ports don't need special cleanup).
func (c *RFCOMMConnection) Close() error {
	return nil
}

// IsDeviceReady checks if the serial device still exists.
func (c *RFCOMMConnection) IsDeviceReady() bool {
	if c.DevicePath == "" {
		return false
	}
	_, err := os.Stat(c.DevicePath)
	return err == nil
}

// GetExistingRFCOMMConnections returns available serial ports on macOS.
func GetExistingRFCOMMConnections() ([]string, error) {
	return ListSerialPorts()
}

// ListSerialPorts enumerates available /dev/cu.* serial ports on macOS.
// cu.* (call-up) devices are preferred over tty.* for outgoing connections.
func ListSerialPorts() ([]string, error) {
	matches, err := filepath.Glob("/dev/cu.*")
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

// FindAvailableRFCOMMDevice returns the first available serial port (for compatibility).
func FindAvailableRFCOMMDevice() (string, int, error) {
	ports, err := ListSerialPorts()
	if err != nil || len(ports) == 0 {
		return "", -1, fmt.Errorf("no serial ports found")
	}
	return ports[0], 0, nil
}
