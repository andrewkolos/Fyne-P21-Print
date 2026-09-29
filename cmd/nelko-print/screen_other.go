//go:build !windows

package main

// screenPxPerMM is only implemented on Windows; 0 falls back to the
// conventional 96 DPI per Fyne scale unit.
func screenPxPerMM() float64 { return 0 }
