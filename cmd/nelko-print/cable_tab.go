package main

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"nelko-print/internal/imaging"
)

// cablePresets are typical outer diameters; the layout tolerates +-1 mm, so
// nobody has to measure a USB cable.
var cablePresets = []struct {
	name string
	mm   float64
}{
	{"USB cable (4 mm)", 4},
	{"Thin USB / earbuds (3 mm)", 3},
	{"Ethernet (6 mm)", 6},
	{"HDMI / power cord (7.5 mm)", 7.5},
}

const customCable = "Custom size..."

// cableTab holds the Cable tab's widgets and settings.
type cableTab struct {
	entry      *widget.Entry
	info       *widget.Label
	mode       imaging.CableMode
	diameterMM float64
	invert     bool
}

func (a *App) buildCableTab() fyne.CanvasObject {
	c := &a.cable
	c.diameterMM = cablePresets[0].mm

	c.entry = widget.NewMultiLineEntry()
	c.entry.SetPlaceHolder("Cable label text...")
	c.entry.SetMinRowsVisible(2)
	c.entry.OnChanged = func(string) { a.updateCablePreview() }

	diameter := widget.NewEntry()
	diameter.SetPlaceHolder("diameter in mm")
	diameter.SetText(strconv.FormatFloat(c.diameterMM, 'g', -1, 64))
	diameter.Hide()
	diameter.OnChanged = func(s string) {
		d, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "mm")), 64)
		if err != nil || d <= 0 {
			d = 0 // RenderCableLabel reports it
		}
		c.diameterMM = d
		a.updateCablePreview()
	}

	names := make([]string, 0, len(cablePresets)+1)
	for _, p := range cablePresets {
		names = append(names, p.name)
	}
	cableType := widget.NewSelect(append(names, customCable), func(s string) {
		if s == customCable {
			diameter.Show()
			diameter.OnChanged(diameter.Text)
			return
		}
		diameter.Hide()
		for _, p := range cablePresets {
			if p.name == s {
				c.diameterMM = p.mm
				diameter.SetText(strconv.FormatFloat(p.mm, 'g', -1, 64))
			}
		}
		a.updateCablePreview()
	})
	cableType.SetSelected(cablePresets[0].name)

	mode := widget.NewRadioGroup([]string{"Wrap", "Flag"}, func(s string) {
		if s == "Flag" {
			c.mode = imaging.CableFlag
		} else {
			c.mode = imaging.CableWrap
		}
		a.updateCablePreview()
	})
	mode.Horizontal = true
	mode.Required = true
	mode.SetSelected("Wrap")

	invert := widget.NewCheck("Invert", func(b bool) {
		c.invert = b
		a.updateCablePreview()
	})

	c.info = widget.NewLabel("")
	c.info.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem("Cable", container.NewVBox(cableType, diameter)),
		widget.NewFormItem("Style", mode),
		widget.NewFormItem("", invert),
	)
	return container.NewVBox(c.entry, form, c.info)
}

func (a *App) updateCablePreview() {
	if a.activeTab != "Cable" {
		return
	}
	c := &a.cable
	if strings.TrimSpace(c.entry.Text) == "" {
		a.sourceImg = nil
		a.clearPreview()
		c.info.SetText("Wrap: text repeats around the cable. Flag: the ends stick together as a tab.")
		return
	}
	img, info, err := imaging.RenderCableLabel(imaging.CableLabel{
		Text:       c.entry.Text,
		Mode:       c.mode,
		DiameterMM: c.diameterMM,
		LabelLenMM: a.labelSize.Height,
		Invert:     c.invert,
	}, a.labelSize.PixelW, a.labelSize.PixelH)
	if err != nil {
		a.sourceImg = nil
		a.clearPreview()
		c.info.SetText(err.Error())
		return
	}
	c.info.SetText(info)
	a.sourceImg = img
	a.updatePreview()
	if a.printer != nil {
		a.printBtn.Enable()
	}
}
