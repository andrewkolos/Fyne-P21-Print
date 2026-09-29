//go:build windows

package main

import (
	"os"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	procMonitorFromWindow    = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW      = user32.NewProc("GetMonitorInfoW")
	procEnumDisplayDevicesW  = user32.NewProc("EnumDisplayDevicesW")
	procEnumDisplaySettingsW = user32.NewProc("EnumDisplaySettingsW")
)

const (
	monitorDefaultToNearest       = 2
	enumCurrentSettings           = 0xFFFFFFFF
	eddGetDeviceInterfaceName     = 1
	dmdoRotated90, dmdoRotated270 = 1, 3
)

type monitorInfoEx struct {
	Size    uint32
	Monitor windows.Rect
	Work    windows.Rect
	Flags   uint32
	Device  [32]uint16
}

type displayDevice struct {
	Size         uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

// devMode is DEVMODEW with the display variant of its unions.
type devMode struct {
	DeviceName                                        [32]uint16
	SpecVersion, DriverVersion, Size, DriverExtra     uint16
	Fields                                            uint32
	PositionX, PositionY                              int32
	DisplayOrientation, DisplayFixedOutput            uint32
	Color, Duplex, YResolution, TTOption, Collate     int16
	FormName                                          [32]uint16
	LogPixels                                         uint16
	BitsPerPel, PelsWidth, PelsHeight                 uint32
	DisplayFlags, DisplayFrequency                    uint32
	ICMMethod, ICMIntent, MediaType, DitherType       uint32
	Reserved1, Reserved2, PanningWidth, PanningHeight uint32
}

var (
	screenMu      sync.Mutex
	appHWND       windows.HWND
	densityByName = map[string]float64{} // \\.\DISPLAYn -> px/mm
)

// screenPxPerMM returns the physical pixel density of the monitor showing
// the app window, from the monitor's EDID size and current mode, or 0 if it
// cannot be determined.
func screenPxPerMM() float64 {
	screenMu.Lock()
	defer screenMu.Unlock()

	if appHWND == 0 || !windows.IsWindowVisible(appHWND) {
		appHWND = findAppWindow()
		if appHWND == 0 {
			return 0
		}
	}
	mon, _, _ := procMonitorFromWindow.Call(uintptr(appHWND), monitorDefaultToNearest)
	if mon == 0 {
		return 0
	}
	mi := monitorInfoEx{Size: uint32(unsafe.Sizeof(monitorInfoEx{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return 0
	}
	name := windows.UTF16ToString(mi.Device[:])
	if d, ok := densityByName[name]; ok {
		return d
	}
	d := monitorDensity(&mi.Device[0])
	densityByName[name] = d
	return d
}

// findAppWindow returns this process's visible GLFW (Fyne) window.
func findAppWindow() windows.HWND {
	foundHWND = 0
	windows.EnumWindows(enumWindowsCallback, nil)
	return foundHWND
}

// Callback slots are never freed, so create the callback once.
var (
	foundHWND           windows.HWND
	enumWindowsCallback = windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var pid uint32
		windows.GetWindowThreadProcessId(hwnd, &pid)
		if pid != uint32(os.Getpid()) || !windows.IsWindowVisible(hwnd) {
			return 1
		}
		buf := make([]uint16, 64)
		n, _ := windows.GetClassName(hwnd, &buf[0], int32(len(buf)))
		if strings.HasPrefix(windows.UTF16ToString(buf[:n]), "GLFW") {
			foundHWND = hwnd
			return 0
		}
		return 1
	})
)

func monitorDensity(device *uint16) float64 {
	dm := devMode{}
	dm.Size = uint16(unsafe.Sizeof(dm))
	if ok, _, _ := procEnumDisplaySettingsW.Call(uintptr(unsafe.Pointer(device)), enumCurrentSettings,
		uintptr(unsafe.Pointer(&dm))); ok == 0 || dm.PelsWidth == 0 {
		return 0
	}

	dd := displayDevice{Size: uint32(unsafe.Sizeof(displayDevice{}))}
	if ok, _, _ := procEnumDisplayDevicesW.Call(uintptr(unsafe.Pointer(device)), 0,
		uintptr(unsafe.Pointer(&dd)), eddGetDeviceInterfaceName); ok == 0 {
		return 0
	}
	wMM, hMM := edidSizeMM(windows.UTF16ToString(dd.DeviceID[:]))
	if wMM <= 0 || hMM <= 0 {
		return 0
	}
	// EDID sizes are for the panel's native (landscape) orientation.
	if dm.DisplayOrientation == dmdoRotated90 || dm.DisplayOrientation == dmdoRotated270 {
		wMM, hMM = hMM, wMM
	}
	return (float64(dm.PelsWidth)/wMM + float64(dm.PelsHeight)/hMM) / 2
}

// edidSizeMM reads the image size from the monitor's EDID. interfaceID looks
// like \\?\DISPLAY#ACI27A7#5&279135e1&0&UID4353#{e6f07b5f-...}.
func edidSizeMM(interfaceID string) (w, h float64) {
	parts := strings.Split(strings.TrimPrefix(interfaceID, `\\?\`), "#")
	if len(parts) < 3 {
		return 0, 0
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Enum\`+parts[0]+`\`+parts[1]+`\`+parts[2]+`\Device Parameters`, registry.READ)
	if err != nil {
		return 0, 0
	}
	defer k.Close()
	e, _, err := k.GetBinaryValue("EDID")
	if err != nil || len(e) < 128 {
		return 0, 0
	}
	// First detailed timing descriptor has the size in mm; bytes 21/22 are cm.
	w = float64(int(e[66]) | int(e[68]>>4)<<8)
	h = float64(int(e[67]) | int(e[68]&0x0F)<<8)
	if w == 0 || h == 0 {
		w, h = float64(e[21])*10, float64(e[22])*10
	}
	return w, h
}
