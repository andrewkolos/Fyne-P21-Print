package printer

import "testing"

func TestParseBattery(t *testing.T) {
	for _, tc := range []struct {
		resp     string
		level    int
		charging bool
	}{
		{"BATTERY \x99\x00\r\n", 99, false}, // captured from a P21
		{"BATTERY \x10\x00\r\n", 10, false}, // level byte is '\n'
		{"BATTERY \x13\x01\r\n", 13, true},  // level byte is '\r'
		{"junkBATTERY \x42\x00\r\n", 42, false},
	} {
		b, err := parseBattery([]byte(tc.resp))
		if err != nil || b.Level != tc.level || b.Charging != tc.charging {
			t.Errorf("%q: got %+v, %v", tc.resp, b, err)
		}
	}
	for _, bad := range []string{"", "BATTERY ", "BATTERY \xff\x00", "CONFIG \x00\xcb"} {
		if _, err := parseBattery([]byte(bad)); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}
