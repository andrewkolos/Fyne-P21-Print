//go:build windows

package printer

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RFCOMMConnection is a compatibility type for Windows
// On Windows, we don't need to manage RFCOMM - COM ports are created automatically
type RFCOMMConnection struct {
	DevicePath string
	MAC        string
}

// ListPairedBluetoothDevices returns the outgoing Bluetooth serial (SPP) COM
// ports on Windows, named after the paired device (e.g. "P21" on COM9).
// BluetoothDevice.MAC holds the COM port, which EstablishRFCOMM expects.
func ListPairedBluetoothDevices() ([]BluetoothDevice, error) {
	devices := namedBluetoothCOMPorts()

	// Fall back to the unnamed \Device\BthModemN entries, then to every port.
	if len(devices) == 0 {
		if btPorts, err := getBluetoothCOMPorts(); err == nil {
			for name, port := range btPorts {
				devices = append(devices, BluetoothDevice{Name: name, MAC: port})
			}
		}
	}
	if len(devices) == 0 {
		ports, _ := ListSerialPorts()
		for _, port := range ports {
			devices = append(devices, BluetoothDevice{Name: port, MAC: port})
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Name != devices[j].Name {
			return devices[i].Name < devices[j].Name
		}
		return comNumber(devices[i].MAC) < comNumber(devices[j].MAC)
	})
	return devices, nil
}

// sppServicePrefix is the BTHENUM key prefix for Serial Port Profile ports.
const sppServicePrefix = "{00001101-0000-1000-8000-00805f9b34fb}"

// namedBluetoothCOMPorts maps each present outgoing SPP COM port to its
// paired device's name via the Bluetooth enumerator's registry entries:
//
//	Enum\BTHENUM\{00001101-...}_LOCALMFG&xxxx\<...&MAC_...>\Device Parameters\PortName
//	Services\BTHPORT\Parameters\Devices\<mac>\Name
//
// Incoming ports (created for other devices to connect to this PC) have an
// all-zero MAC and are skipped; so are ports of since-removed devices.
func namedBluetoothCOMPorts() []BluetoothDevice {
	present := make(map[string]bool)
	if ports, err := ListSerialPorts(); err == nil {
		for _, p := range ports {
			present[strings.ToUpper(p)] = true
		}
	}

	enum, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Enum\BTHENUM`, registry.READ)
	if err != nil {
		return nil
	}
	defer enum.Close()
	services, err := enum.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}

	var devices []BluetoothDevice
	for _, svc := range services {
		if !strings.HasPrefix(strings.ToLower(svc), sppServicePrefix) {
			continue
		}
		svcKey, err := registry.OpenKey(enum, svc, registry.READ)
		if err != nil {
			continue
		}
		instances, _ := svcKey.ReadSubKeyNames(-1)
		for _, inst := range instances {
			mac := instanceMAC(inst)
			if mac == "" || strings.Trim(mac, "0") == "" {
				continue
			}
			params, err := registry.OpenKey(svcKey, inst+`\Device Parameters`, registry.READ)
			if err != nil {
				continue
			}
			port, _, err := params.GetStringValue("PortName")
			params.Close()
			if err != nil || !present[strings.ToUpper(port)] {
				continue
			}
			name := bluetoothDeviceName(mac)
			if name == "" {
				name = formatMAC(mac)
			}
			devices = append(devices, BluetoothDevice{Name: name, MAC: port})
		}
		svcKey.Close()
	}
	return devices
}

// instanceMAC extracts the 12-hex-digit device address from a BTHENUM
// instance name such as "a&14fc963d&0&956DE02578AD_C00000000".
func instanceMAC(inst string) string {
	part := inst
	if i := strings.LastIndex(part, "&"); i >= 0 {
		part = part[i+1:]
	}
	if i := strings.Index(part, "_"); i >= 0 {
		part = part[:i]
	}
	if len(part) != 12 {
		return ""
	}
	for _, c := range part {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return ""
		}
	}
	return strings.ToLower(part)
}

// bluetoothDeviceName reads a paired device's name (NUL-terminated UTF-8).
func bluetoothDeviceName(mac string) string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\BTHPORT\Parameters\Devices\`+mac, registry.READ)
	if err != nil {
		return ""
	}
	defer k.Close()
	b, _, err := k.GetBinaryValue("Name")
	if err != nil {
		return ""
	}
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}

func formatMAC(mac string) string {
	var parts []string
	for i := 0; i+2 <= len(mac); i += 2 {
		parts = append(parts, strings.ToUpper(mac[i:i+2]))
	}
	return strings.Join(parts, ":")
}

// comNumber returns N for "COMN" so COM10 sorts after COM9.
func comNumber(port string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(port), "COM"))
	return n
}

// getBluetoothCOMPorts reads Bluetooth COM port mappings from registry
func getBluetoothCOMPorts() (map[string]string, error) {
	ports := make(map[string]string)

	// Try to read from SERIALCOMM registry key
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DEVICEMAP\SERIALCOMM`, registry.READ)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		val, _, err := key.GetStringValue(name)
		if err == nil {
			// Check if it looks like a Bluetooth port
			if strings.Contains(strings.ToLower(name), "bth") ||
				strings.Contains(strings.ToLower(name), "bluetooth") {
				ports[name] = val
			}
		}
	}

	return ports, nil
}

// CheckRFCOMMInstalled always returns nil on Windows (not needed)
func CheckRFCOMMInstalled() error {
	return nil
}

// CheckPrivilegeHelper always returns "windows" on Windows (no elevation needed for COM)
func CheckPrivilegeHelper() string {
	return "windows"
}

// EstablishRFCOMM on Windows simply returns the COM port path
// Windows handles BT SPP as regular COM ports, no special setup needed
func EstablishRFCOMM(mac string, channel int, statusCallback func(string)) (*RFCOMMConnection, error) {
	// On Windows, 'mac' is actually the COM port (e.g., "COM3")
	if statusCallback != nil {
		statusCallback(fmt.Sprintf("Using port %s...", mac))
	}

	// Verify the port exists
	comPath := mac
	if !strings.HasPrefix(strings.ToUpper(mac), "COM") {
		return nil, fmt.Errorf("invalid COM port: %s", mac)
	}

	// For COM ports > 9, need to use \\.\COM10 format
	if len(mac) > 4 {
		comPath = `\\.\` + mac
	}

	conn := &RFCOMMConnection{
		DevicePath: comPath,
		MAC:        mac,
	}

	if statusCallback != nil {
		statusCallback(fmt.Sprintf("Ready: %s", mac))
	}

	return conn, nil
}

// Close is a no-op on Windows (COM ports don't need special cleanup)
func (c *RFCOMMConnection) Close() error {
	return nil
}

// IsDeviceReady checks if the COM port exists
func (c *RFCOMMConnection) IsDeviceReady() bool {
	if c.DevicePath == "" {
		return false
	}
	// On Windows, we can't easily check if a COM port is available
	// without trying to open it, so we just return true
	return true
}

// GetExistingRFCOMMConnections returns available COM ports on Windows
func GetExistingRFCOMMConnections() ([]string, error) {
	return ListSerialPorts()
}

// ListSerialPorts enumerates available COM ports on Windows
func ListSerialPorts() ([]string, error) {
	var ports []string

	// Read from registry
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DEVICEMAP\SERIALCOMM`, registry.READ)
	if err != nil {
		// Fallback: check common COM ports
		for i := 1; i <= 20; i++ {
			port := fmt.Sprintf("COM%d", i)
			// Try to check if port exists (this is a rough check)
			ports = append(ports, port)
		}
		return ports[:4], nil // Return first 4 as fallback
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		val, _, err := key.GetStringValue(name)
		if err == nil {
			ports = append(ports, val)
		}
	}

	// Registry order is arbitrary; keep the dropdown stable.
	sort.Slice(ports, func(i, j int) bool { return comNumber(ports[i]) < comNumber(ports[j]) })

	return ports, nil
}

// FindAvailableRFCOMMDevice is not needed on Windows but provided for compatibility
func FindAvailableRFCOMMDevice() (string, int, error) {
	ports, err := ListSerialPorts()
	if err != nil || len(ports) == 0 {
		return "", -1, fmt.Errorf("no COM ports found")
	}
	return ports[0], 0, nil
}
