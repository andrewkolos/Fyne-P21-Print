package imaging

import (
	"image"
	"image/color"
	"testing"
)

func TestSimulatePrintActualSize(t *testing.T) {
	dots := image.NewGray(image.Rect(0, 0, 284, 96))
	for i := range dots.Pix {
		dots.Pix[i] = 255
	}
	dots.SetGray(142, 48, color.Gray{0}) // one dot in the middle

	img := SimulatePrint(dots, 40, 14, 4.287).(*image.RGBA)
	if got := img.Bounds().Size(); got != image.Pt(171, 60) {
		t.Fatalf("size %v, want 171x60 (40x14 mm at 4.287 px/mm)", got)
	}
	if a := img.RGBAAt(0, 0).A; a != 0 {
		t.Errorf("corner alpha %d, want 0 (rounded)", a)
	}
	if c := img.RGBAAt(10, 30); c.A != 255 || c.R != 255 {
		t.Errorf("blank paper %v, want opaque white", c)
	}
	// A single dot is smaller than a screen pixel: it must show as grey,
	// neither vanishing nor as a solid black pixel.
	if c := img.RGBAAt(85, 30); c.R >= 250 || c.R <= inkGray {
		t.Errorf("single dot rendered %v, want a mid grey", c)
	}
}
