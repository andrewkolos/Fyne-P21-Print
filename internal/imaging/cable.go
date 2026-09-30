package imaging

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

// CableMode selects how a cable label is laid out.
type CableMode int

const (
	// CableWrap: the label wraps around the cable. The text runs along the
	// cable (across the tape) and repeats around it, so it reads from any side.
	CableWrap CableMode = iota
	// CableFlag: the middle wraps the cable and both ends stick back to back
	// as a flag; the text is printed on each end.
	CableFlag
)

const (
	wrapCopiesAround = 3   // text copies around one turn of the cable
	minWrapPitchMM   = 3.0 // smallest repeat worth printing
	minFlagMM        = 6.0 // shortest useful flag half
	flagSlackMM      = 0.5 // extra wrap length so the ends meet around the cable
	foldTickDots     = 6
)

// CableLabel describes a cable label on a die-cut label whose print area
// (width x height dots, height along the label) is centred on its length.
type CableLabel struct {
	Text       string
	Mode       CableMode
	DiameterMM float64
	LabelLenMM float64
	Invert     bool
}

// RenderCableLabel renders the label at print resolution (width dots across
// the print head, height dots along the label) and returns a one-line summary
// of the layout for the UI.
func RenderCableLabel(c CableLabel, width, height int) (image.Image, string, error) {
	if c.DiameterMM <= 0 {
		return nil, "", errors.New("enter the cable diameter in mm")
	}
	circ := math.Pi * c.DiameterMM
	printLenMM := float64(height) / dotsPerMM
	offsetMM := (c.LabelLenMM - printLenMM) / 2 // label edge to print area

	bg, fg := color.Gray{255}, color.Gray{0}
	if c.Invert {
		bg, fg = fg, bg
	}
	img := image.NewGray(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	opts := TextOptions{WordBreakOnly: true, Invert: c.Invert}

	switch c.Mode {
	case CableFlag:
		flagMM := (c.LabelLenMM - circ - flagSlackMM) / 2
		if flagMM < minFlagMM {
			maxD := (c.LabelLenMM - 2*minFlagMM - flagSlackMM) / math.Pi
			return nil, "", fmt.Errorf("too thick for a flag on a %.0f mm label (max %.1f mm cable); use Wrap or a longer label",
				c.LabelLenMM, maxD)
		}
		// Label-mm to dot row, clamped to the print area.
		row := func(mm float64) int {
			return max(0, min(height, int(math.Round((mm-offsetMM)*dotsPerMM))))
		}
		regions := [][2]int{{row(0), row(flagMM)}, {row(c.LabelLenMM - flagMM), row(c.LabelLenMM)}}
		for _, r := range regions {
			opts.Orientation = Vertical // along the label, like the Text tab
			cell, err := RenderTextWithOptions(c.Text, width, r[1]-r[0], opts)
			if err != nil {
				return nil, "", err
			}
			draw.Draw(img, image.Rect(0, r[0], width, r[1]), cell, cell.Bounds().Min, draw.Src)
		}
		// Fold marks at both edges where the flag halves meet the cable.
		for _, y := range []int{regions[0][1], regions[1][0]} {
			for dy := -1; dy <= 0; dy++ {
				for dx := 0; dx < foldTickDots; dx++ {
					img.SetGray(dx, y+dy, fg)
					img.SetGray(width-1-dx, y+dy, fg)
				}
			}
		}
		return img, fmt.Sprintf("Flag: %.1f mm tab, %.1f mm wraps the %g mm cable. Fold at the edge marks.",
			flagMM, circ+flagSlackMM, c.DiameterMM), nil

	default:
		pitchMM := math.Max(minWrapPitchMM, math.Min(circ/wrapCopiesAround, printLenMM))
		pitch := int(math.Round(pitchMM * dotsPerMM))
		n := max(1, height/pitch)
		opts.Orientation = Horizontal // across the tape = along the cable
		cell, err := RenderTextWithOptions(c.Text, width, pitch, opts)
		if err != nil {
			return nil, "", err
		}
		y0 := (height - n*pitch) / 2
		for i := 0; i < n; i++ {
			y := y0 + i*pitch
			draw.Draw(img, image.Rect(0, y, width, y+pitch), cell, cell.Bounds().Min, draw.Src)
		}
		around := circ / pitchMM
		return img, fmt.Sprintf("Wrap: goes %.1f× around a %g mm cable, text every %.1f mm (%.1f× around).",
			c.LabelLenMM/circ, c.DiameterMM, pitchMM, around), nil
	}
}
